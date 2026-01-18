import argparse
import io
import json
import os
import pathlib
import random
import time
from typing import Any

import cv2
import faiss
import numpy as np
import requests
import torch
from PIL import Image
from transformers import CLIPModel, CLIPProcessor

from identifier.set_matcher import CropExtractor
from identifier.warp import warp_card


def fetch_sets() -> list[dict[str, Any]]:
    """Fetch all MTG sets from Scryfall with metadata."""
    resp = requests.get("https://api.scryfall.com/sets", timeout=60)
    resp.raise_for_status()
    data = resp.json()
    # Filter to sets that have cards (not tokens-only sets)
    return [s for s in data.get("data", []) if s.get("code") and s.get("card_count", 0) > 0]


def fetch_card_images(set_code: str, limit: int, diverse: bool = True) -> list[bytes]:
    """Fetch card images from Scryfall API with optional diversity sampling.

    Args:
        set_code: The set code to fetch cards from.
        limit: Number of cards to return.
        diverse: If True, sample from different card types for variety.

    Returns:
        List of image bytes.
    """
    url = "https://api.scryfall.com/cards/search"
    # Fetch more cards than needed if diverse sampling
    fetch_count = limit * 3 if diverse else limit
    params = {"q": f"set:{set_code}", "unique": "prints"}

    resp = requests.get(url, params=params, timeout=60)
    if resp.status_code != 200:
        return []
    payload = resp.json()

    cards = payload.get("data", [])
    if not cards:
        return []

    # Diverse sampling: pick different card types
    if diverse and len(cards) > limit:
        # Categorize cards by type
        creatures = [c for c in cards if "Creature" in c.get("type_line", "")]
        spells = [c for c in cards if "Creature" not in c.get("type_line", "") and "Land" not in c.get("type_line", "")]
        lands = [c for c in cards if "Land" in c.get("type_line", "")]

        selected = []
        # Try to include a mix of card types
        for category in [creatures, spells, lands]:
            if category:
                selected.extend(random.sample(category, min(2, len(category))))

        # If we still need more, fill from all cards
        if len(selected) < limit:
            remaining_cards = [c for c in cards if c not in selected]
            if remaining_cards:
                selected.extend(random.sample(remaining_cards, min(limit - len(selected), len(remaining_cards))))

        cards = selected[:limit]

    images: list[bytes] = []
    for card in cards[:limit]:
        img_url = None
        if card.get("image_uris"):
            img_url = card["image_uris"].get("large") or card["image_uris"].get("normal")
        elif card.get("card_faces"):
            face = card["card_faces"][0]
            if face.get("image_uris"):
                img_url = face["image_uris"].get("large") or face["image_uris"].get("normal")
        if not img_url:
            continue
        try:
            img = requests.get(img_url, timeout=60)
            if img.status_code == 200:
                images.append(img.content)
        except requests.RequestException:
            continue

    return images


@torch.no_grad()
def embed_images(model: CLIPModel, processor: CLIPProcessor, device: torch.device, imgs: list[Image.Image]) -> np.ndarray:
    inputs = processor(images=imgs, return_tensors="pt")
    inputs = {k: v.to(device) for k, v in inputs.items()}
    feats = model.get_image_features(**inputs)
    feats = feats / feats.norm(dim=-1, keepdim=True)
    return feats.detach().cpu().numpy().astype("float32")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--per-set", type=int, default=6)
    ap.add_argument("--min-coverage", type=float, default=0.90, help="Minimum set coverage (0-1)")
    args = ap.parse_args()

    out_dir = pathlib.Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    print(f"Using device: {device}")

    model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32").to(device)
    processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")
    model.eval()

    cropper = CropExtractor()

    sets_data = fetch_sets()
    total_sets = len(sets_data)
    print(f"Found {total_sets} MTG sets to index")

    vectors: list[np.ndarray] = []
    meta: list[dict[str, Any]] = []

    # Track indexed and failed sets for validation
    indexed_sets: set[str] = set()
    failed_sets: list[dict[str, Any]] = []

    for i, set_info in enumerate(sets_data):
        code = set_info["code"]
        set_name = set_info.get("name", code)

        try:
            imgs_bytes = fetch_card_images(code, args.per_set, diverse=True)
        except Exception as e:
            failed_sets.append({"set_id": code, "name": set_name, "error": str(e), "stage": "fetch"})
            print(f"[{i+1}/{total_sets}] FAILED to fetch {code}: {e}")
            continue

        if not imgs_bytes:
            failed_sets.append({"set_id": code, "name": set_name, "error": "no images returned", "stage": "fetch"})
            print(f"[{i+1}/{total_sets}] FAILED {code}: no images returned")
            continue

        pil_imgs: list[Image.Image] = []
        for b in imgs_bytes:
            try:
                pil_imgs.append(Image.open(io.BytesIO(b)).convert("RGB"))
            except Exception:
                continue

        if not pil_imgs:
            failed_sets.append({"set_id": code, "name": set_name, "error": "failed to decode images", "stage": "decode"})
            print(f"[{i+1}/{total_sets}] FAILED {code}: failed to decode images")
            continue

        cropped_pil: list[Image.Image] = []
        for img in pil_imgs:
            try:
                bgr = cv2.cvtColor(np.array(img), cv2.COLOR_RGB2BGR)
                warped, _ = warp_card(bgr)
                crops, _ = cropper.extract(game="mtg", warped_bgr=warped)
                cropped_pil.extend(crops)
            except Exception:
                continue

        if not cropped_pil:
            failed_sets.append({"set_id": code, "name": set_name, "error": "failed to extract crops", "stage": "crop"})
            print(f"[{i+1}/{total_sets}] FAILED {code}: failed to extract crops")
            continue

        embs = embed_images(model, processor, device, cropped_pil)
        for row in embs:
            vectors.append(row)
            meta.append({"set_id": code})

        indexed_sets.add(code)
        print(f"[{i+1}/{total_sets}] Indexed {code} ({set_name}): {len(cropped_pil)} crops")

        # Scryfall rate limiting (50ms between requests)
        if i % 40 == 0 and i > 0:
            time.sleep(0.2)

    if not vectors:
        raise SystemExit("no vectors built")

    # Build and save index
    mat = np.vstack(vectors)
    dim = mat.shape[1]
    index = faiss.IndexFlatL2(dim)
    index.add(mat)

    faiss.write_index(index, str(out_dir / "mtg.faiss"))
    with open(out_dir / "mtg_meta.json", "w", encoding="utf-8") as f:
        json.dump(meta, f)

    # Generate validation report
    coverage = len(indexed_sets) / total_sets if total_sets > 0 else 0
    report = {
        "game": "mtg",
        "total_sets": total_sets,
        "indexed_sets": len(indexed_sets),
        "failed_sets_count": len(failed_sets),
        "coverage_pct": round(coverage * 100, 2),
        "total_vectors": len(vectors),
        "vectors_per_set_avg": round(len(vectors) / len(indexed_sets), 2) if indexed_sets else 0,
        "per_set_requested": args.per_set,
        "failed_sets": failed_sets,
        "indexed_set_ids": sorted(indexed_sets),
    }

    with open(out_dir / "mtg_validation_report.json", "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2)

    # Print summary
    print("\n" + "=" * 60)
    print("MTG INDEX BUILD SUMMARY")
    print("=" * 60)
    print(f"Total sets:     {total_sets}")
    print(f"Indexed sets:   {len(indexed_sets)}")
    print(f"Failed sets:    {len(failed_sets)}")
    print(f"Coverage:       {coverage * 100:.1f}%")
    print(f"Total vectors:  {len(vectors)}")
    print(f"Avg per set:    {len(vectors) / len(indexed_sets):.1f}" if indexed_sets else "N/A")
    print("=" * 60)

    # Check minimum coverage requirement
    if coverage < args.min_coverage:
        print(f"\nERROR: Coverage {coverage * 100:.1f}% is below minimum {args.min_coverage * 100:.1f}%")
        raise SystemExit(1)

    print(f"\nIndex saved to {out_dir}")
    print(f"Validation report saved to {out_dir / 'mtg_validation_report.json'}")


if __name__ == "__main__":
    main()
