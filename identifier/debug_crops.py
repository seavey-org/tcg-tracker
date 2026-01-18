from __future__ import annotations

import argparse
import sys
from pathlib import Path

# Allow running as a script from repo root:
# `python identifier/debug_crops.py ...`
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

import cv2
import numpy as np

from identifier.set_matcher import CropExtractor
from identifier.warp import warp_card


def _ensure_empty_dir(path: Path) -> None:
    if path.exists():
        for p in path.rglob("*"):
            if p.is_file():
                p.unlink()
        for p in sorted([p for p in path.rglob("*") if p.is_dir()], reverse=True):
            p.rmdir()
    path.mkdir(parents=True, exist_ok=True)


def main() -> None:
    parser = argparse.ArgumentParser(description="Debug set icon crops")
    parser.add_argument("--game", default="pokemon", choices=["pokemon", "mtg"])
    parser.add_argument(
        "--input-dir",
        default=str(Path("backend/internal/services/testdata/pokemon_cards")),
        help="Directory of card photos to process",
    )
    parser.add_argument(
        "--out-dir",
        default=str(Path("identifier/_debug_crops")),
        help="Directory to write crops/debug images",
    )
    parser.add_argument("--no-warp", action="store_true", help="Skip warp_card and use raw image")
    parser.add_argument(
        "--warp-size",
        default="744x1040",
        help="Warp output size as WxH (default 744x1040)",
    )
    args = parser.parse_args()

    try:
        warp_w_s, warp_h_s = args.warp_size.lower().split("x", 1)
        warp_size = (int(warp_w_s), int(warp_h_s))
    except Exception:
        raise SystemExit(f"invalid --warp-size: {args.warp_size} (expected WxH, e.g. 744x1040)")

    input_dir = Path(args.input_dir)
    out_dir = Path(args.out_dir)

    if not input_dir.exists() or not input_dir.is_dir():
        raise SystemExit(f"input dir not found: {input_dir}")

    _ensure_empty_dir(out_dir)

    extractor = CropExtractor()

    exts = {".png", ".jpg", ".jpeg", ".webp"}
    images = [p for p in sorted(input_dir.iterdir()) if p.suffix.lower() in exts]

    if not images:
        raise SystemExit(f"no images found in {input_dir}")

    for img_path in images:
        bgr = cv2.imread(str(img_path), cv2.IMREAD_COLOR)
        if bgr is None:
            print(f"skip (failed to read): {img_path}")
            continue

        if args.no_warp:
            warped = bgr
            warp_debug = {"found_quad": False, "fallback": "no-warp"}
        else:
            warped, warp_debug = warp_card(bgr, out_size=warp_size)

        crops, crop_debug = extractor.extract(game=args.game, warped_bgr=warped)

        stem = img_path.stem
        card_dir = out_dir / stem
        card_dir.mkdir(parents=True, exist_ok=True)

        cv2.imwrite(str(card_dir / "00_input.png"), bgr)
        cv2.imwrite(str(card_dir / "01_warped.png"), warped)

        # Write crops
        for i, pil in enumerate(crops):
            rgb = pil.convert("RGB")
            arr_rgb = np.array(rgb)
            arr_bgr = cv2.cvtColor(arr_rgb, cv2.COLOR_RGB2BGR)
            cv2.imwrite(str(card_dir / f"crop_{i:02d}.png"), arr_bgr)

        # Draw boxes on warped for quick sanity checks
        boxed = warped.copy()
        for b in crop_debug.get("boxes", []):
            x1 = int(b["x"])
            y1 = int(b["y"])
            x2 = x1 + int(b["w"])
            y2 = y1 + int(b["h"])
            cv2.rectangle(boxed, (x1, y1), (x2, y2), (0, 255, 0), 2)
        cv2.imwrite(str(card_dir / "02_warped_boxes.png"), boxed)

        with open(card_dir / "debug.txt", "w", encoding="utf-8") as f:
            f.write(f"image={img_path.name}\n")
            f.write(f"warp_debug={warp_debug}\n")
            f.write(f"num_crops={len(crops)}\n")
            f.write(f"boxes={crop_debug.get('boxes', [])}\n")

        print(f"wrote {stem}: {len(crops)} crops")


if __name__ == "__main__":
    main()
