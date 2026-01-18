from __future__ import annotations

import json
import os
import time
from dataclasses import dataclass
from typing import Any

import cv2
import numpy as np
from PIL import Image

# Optional heavy deps; CropExtractor is usable without them.
try:
    import faiss  # type: ignore
except Exception:  # pragma: no cover
    faiss = None

try:
    import torch  # type: ignore
    from transformers import CLIPModel, CLIPProcessor  # type: ignore
except Exception:  # pragma: no cover
    torch = None
    CLIPModel = None
    CLIPProcessor = None


@dataclass
class LoadedIndex:
    index: Any
    meta: list[dict[str, Any]]


def _cosine_from_l2_distance(d: float) -> float:
    # If embeddings are normalized, FAISS IndexFlatL2 returns squared L2 distance.
    # For unit vectors: ||a-b||^2 = 2 - 2cos => cos = 1 - d/2
    return 1.0 - (d / 2.0)


class CropExtractor:
    def extract(self, game: str, warped_bgr: np.ndarray) -> tuple[list[Image.Image], dict[str, Any]]:
        h, w = warped_bgr.shape[:2]

        crops: list[tuple[tuple[int, int, int, int], np.ndarray]] = []

        if game == "pokemon":
            # Pokemon set symbols/codes appear in different locations depending on card era:
            #
            # WOTC-ERA (1999-2003) - Middle-right of artwork:
            # - Base Set, Jungle, Fossil, Team Rocket, Gym, Neo series
            # - Set symbol appears on right side near artwork border
            # - These symbols are larger and more prominent
            #
            # E-CARD ERA (2001-2003) - Bottom center:
            # - Expedition, Aquapolis, Skyridge
            # - Set symbol appears in bottom center region with e-Reader dot code
            #
            # EX/DP ERA (2003-2011) - Bottom-right corner:
            # - EX series, Diamond & Pearl, Platinum, HeartGold & SoulSilver
            # - Set symbol moved to bottom-right corner below attacks
            #
            # MODERN (2011+) - Bottom-left collector info:
            # - Black & White, XY, Sun & Moon, Sword & Shield, Scarlet & Violet
            # - Set symbol/code appears in bottom-left with collector number
            #
            # PROMOS - Various locations:
            # - Black Star promos may have symbols in top-right or middle-right
            # - Special promos can have unique placements
            #
            rois = [
                # === WOTC-ERA: Middle-right artwork border (larger, prominent symbols) ===
                (int(0.80 * w), int(0.52 * h), int(0.96 * w), int(0.66 * h)),
                (int(0.75 * w), int(0.48 * h), int(0.98 * w), int(0.70 * h)),
                (int(0.78 * w), int(0.45 * h), int(0.95 * w), int(0.60 * h)),

                # === E-CARD ERA: Bottom center region (e-Reader cards) ===
                (int(0.40 * w), int(0.92 * h), int(0.60 * w), int(0.98 * h)),
                (int(0.35 * w), int(0.90 * h), int(0.65 * w), int(0.99 * h)),
                (int(0.30 * w), int(0.88 * h), int(0.70 * w), int(0.98 * h)),

                # === EX/DP ERA: Bottom-right corner ===
                (int(0.75 * w), int(0.88 * h), int(0.98 * w), int(0.98 * h)),
                (int(0.70 * w), int(0.85 * h), int(0.98 * w), int(0.99 * h)),
                (int(0.80 * w), int(0.90 * h), int(0.96 * w), int(0.97 * h)),

                # === MODERN: Bottom-left collector info region ===
                (int(0.01 * w), int(0.89 * h), int(0.15 * w), int(0.97 * h)),
                (int(0.01 * w), int(0.87 * h), int(0.35 * w), int(0.98 * h)),
                (int(0.01 * w), int(0.90 * h), int(0.10 * w), int(0.96 * h)),

                # === PROMOS/SPECIAL: Top-right corner (Black Star promos) ===
                (int(0.85 * w), int(0.02 * h), int(0.98 * w), int(0.08 * h)),
                (int(0.80 * w), int(0.01 * h), int(0.99 * w), int(0.10 * h)),

                # === FULL ART/SPECIAL: Alternative bottom regions ===
                (int(0.01 * w), int(0.92 * h), int(0.20 * w), int(0.99 * h)),
            ]
        else:
            # MTG set symbols appear in different locations based on card frame style:
            #
            # MODERN FRAME (2003+) - Middle-right below art:
            # - Set symbol appears on right side between art and text box
            #
            # OLD FRAME (pre-2003) - Middle-right:
            # - Similar position but different frame aesthetics
            #
            # BORDERLESS/SHOWCASE - Various positions:
            # - May have symbols in non-standard locations
            #
            rois = [
                # === STANDARD: Middle-right set symbol region ===
                (int(0.60 * w), int(0.42 * h), int(0.98 * w), int(0.58 * h)),
                (int(0.65 * w), int(0.45 * h), int(0.95 * w), int(0.55 * h)),
                (int(0.60 * w), int(0.35 * h), int(0.98 * w), int(0.70 * h)),

                # === RARITY/COLLECTOR INFO: Bottom region ===
                (int(0.70 * w), int(0.88 * h), int(0.98 * w), int(0.98 * h)),
                (int(0.01 * w), int(0.90 * h), int(0.30 * w), int(0.98 * h)),

                # === EXTENDED ART/BORDERLESS: Alternative regions ===
                (int(0.75 * w), int(0.38 * h), int(0.99 * w), int(0.62 * h)),
                (int(0.55 * w), int(0.40 * h), int(0.85 * w), int(0.60 * h)),

                # === OLD FRAME: Slightly different positioning ===
                (int(0.58 * w), int(0.48 * h), int(0.92 * w), int(0.56 * h)),
            ]

        for (x1, y1, x2, y2) in rois:
            roi = warped_bgr[y1:y2, x1:x2]
            crops.append(((x1, y1, x2, y2), roi))

        # Generate contour crops from the biggest (catch-all) ROI to better isolate
        # the actual set symbol.
        primary = crops[-1][1]
        primary_xy = crops[-1][0]
        contour_crops = self._contour_proposals(primary, primary_xy)
        crops.extend(contour_crops[:6])

        pil_crops: list[Image.Image] = []
        debug: dict[str, Any] = {"boxes": []}

        # Keep more crops; embedding+search will de-dup by set id later.
        max_crops = 10
        for (x1, y1, x2, y2), crop_bgr in crops[:max_crops]:
            rgb = cv2.cvtColor(crop_bgr, cv2.COLOR_BGR2RGB)
            pil = Image.fromarray(rgb)
            pil_crops.append(pil)
            debug["boxes"].append({"x": x1, "y": y1, "w": x2 - x1, "h": y2 - y1})

        return pil_crops, debug

    def _contour_proposals(self, roi_bgr: np.ndarray, roi_xy: tuple[int, int, int, int]) -> list[tuple[tuple[int, int, int, int], np.ndarray]]:
        x0, y0, _, _ = roi_xy
        gray = cv2.cvtColor(roi_bgr, cv2.COLOR_BGR2GRAY)
        gray = cv2.GaussianBlur(gray, (3, 3), 0)
        thr = cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY_INV, 31, 5)

        contours, _ = cv2.findContours(thr, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

        proposals: list[tuple[tuple[int, int, int, int], np.ndarray, float]] = []
        for cnt in contours:
            x, y, w, h = cv2.boundingRect(cnt)
            area = w * h

            # Filter for "icon-sized" blobs (set symbols are typically modest in size)
            if area < 250 or area > 25000:
                continue
            if w < 12 or h < 12:
                continue

            aspect = w / float(h)
            # Set icons are usually close to square-ish.
            if aspect < 0.4 or aspect > 2.2:
                continue

            fill = cv2.contourArea(cnt) / float(area)
            if fill < 0.20:
                continue

            pad = 8
            x1 = max(0, x - pad)
            y1 = max(0, y - pad)
            x2 = min(roi_bgr.shape[1], x + w + pad)
            y2 = min(roi_bgr.shape[0], y + h + pad)

            crop = roi_bgr[y1:y2, x1:x2]
            proposals.append(((x0 + x1, y0 + y1, x0 + x2, y0 + y2), crop, area))

        proposals.sort(key=lambda t: t[2], reverse=True)
        return [p[:2] for p in proposals]


class SetMatcher:
    def __init__(self, index_dir: str) -> None:
        if faiss is None or torch is None or CLIPModel is None or CLIPProcessor is None:
            raise RuntimeError(
                "missing dependencies for SetMatcher; install identifier/requirements.txt (faiss, torch, transformers)"
            )

        self.index_dir = index_dir
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

        self.model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32")
        self.processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")
        self.model.to(self.device)
        self.model.eval()

        self._indexes: dict[str, LoadedIndex] = {}
        if index_dir:
            for game in ("pokemon", "mtg"):
                self._try_load(game)

    def games_loaded(self) -> list[str]:
        return sorted(self._indexes.keys())

    def _try_load(self, game: str) -> None:
        faiss_path = os.path.join(self.index_dir, f"{game}.faiss")
        meta_path = os.path.join(self.index_dir, f"{game}_meta.json")
        if not os.path.exists(faiss_path) or not os.path.exists(meta_path):
            return

        if faiss is None:
            raise RuntimeError("faiss not installed")
        index = faiss.read_index(faiss_path)
        with open(meta_path, "r", encoding="utf-8") as f:
            meta = json.load(f)

        self._indexes[game] = LoadedIndex(index=index, meta=meta)

    def identify(self, game: str, warped_bgr: np.ndarray, k: int = 20) -> dict[str, Any]:
        if game not in self._indexes:
            raise RuntimeError(f"index for game {game} not loaded")

        t0 = time.time()
        crops, crop_debug = CropExtractor().extract(game=game, warped_bgr=warped_bgr)
        crop_debug["per_crop_top"] = []
        t_crop = int((time.time() - t0) * 1000)

        if not crops:
            return {
                "best_set_id": "",
                "confidence": 0.0,
                "low_confidence": True,
                "candidates": [],
                "timings_ms": {"crop": t_crop},
                "debug": {"crops": crop_debug},
            }

        t1 = time.time()
        embeddings = self._embed(crops)
        t_embed = int((time.time() - t1) * 1000)

        t2 = time.time()
        candidates, per_crop_top = self._search(game, embeddings, k=k)
        crop_debug["per_crop_top"] = per_crop_top
        t_search = int((time.time() - t2) * 1000)

        best = candidates[0] if candidates else {"set_id": "", "score": 0.0}

        confidence, low_conf = self._confidence(candidates=candidates, per_crop_top=crop_debug.get("per_crop_top", []))

        out: dict[str, Any] = {
            "best_set_id": best["set_id"],
            "confidence": confidence,
            "low_confidence": low_conf,
            "candidates": candidates,
            "timings_ms": {"crop": t_crop, "embed": t_embed, "search": t_search},
            "debug": {"crops": crop_debug},
        }
        return out

    def _contour_proposals(self, roi_bgr: np.ndarray, roi_xy: tuple[int, int, int, int]) -> list[tuple[tuple[int, int, int, int], np.ndarray]]:
        return CropExtractor()._contour_proposals(roi_bgr, roi_xy)

    def _embed(self, images: list[Image.Image]) -> np.ndarray:
        if torch is None:
            raise RuntimeError("torch not installed")

        with torch.no_grad():
            inputs = self.processor(images=images, return_tensors="pt")
            inputs = {k: v.to(self.device) for k, v in inputs.items()}

            feats = self.model.get_image_features(**inputs)
            feats = feats / feats.norm(dim=-1, keepdim=True)
            return feats.detach().cpu().numpy().astype("float32")

    def _search(self, game: str, embeddings: np.ndarray, k: int) -> tuple[list[dict[str, Any]], list[str]]:
        loaded = self._indexes[game]

        # Use L2 on unit vectors; convert distance to cosine score.
        dists, idxs = loaded.index.search(embeddings, k)

        per_set_best: dict[str, float] = {}
        for crop_i in range(idxs.shape[0]):
            for j in range(idxs.shape[1]):
                row = int(idxs[crop_i, j])
                if row < 0:
                    continue
                set_id = loaded.meta[row]["set_id"]
                score = _cosine_from_l2_distance(float(dists[crop_i, j]))
                prev = per_set_best.get(set_id)
                if prev is None or score > prev:
                    per_set_best[set_id] = score

        items = [{"set_id": sid, "score": float(score)} for sid, score in per_set_best.items()]
        items.sort(key=lambda x: x["score"], reverse=True)
        topk = items[:k]

        per_crop_top: list[str] = []
        for crop_i in range(idxs.shape[0]):
            row = int(idxs[crop_i, 0]) if idxs.shape[1] > 0 else -1
            if row >= 0:
                per_crop_top.append(str(loaded.meta[row]["set_id"]))

        return topk, per_crop_top

    def _confidence(self, candidates: list[dict[str, Any]], per_crop_top: list[str]) -> tuple[float, bool]:
        if not candidates:
            return 0.0, True

        top1 = float(candidates[0]["score"])
        top2 = float(candidates[1]["score"]) if len(candidates) > 1 else 0.0
        margin = top1 - top2

        best_set = candidates[0]["set_id"]
        stability = 1.0
        if per_crop_top:
            agree = sum(1 for s in per_crop_top if s == best_set)
            stability = agree / float(len(per_crop_top))

        low_conf = top1 < 0.28 or margin < 0.03 or stability < 0.60

        # Confidence is a bounded blend of top score and margin.
        conf = max(0.0, min(1.0, (top1 - 0.15) * 1.2))
        conf = max(conf, max(0.0, min(1.0, margin * 10.0)))

        return conf, low_conf
