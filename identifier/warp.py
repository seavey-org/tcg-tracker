from __future__ import annotations

from typing import Any

import cv2
import numpy as np


def _order_points(pts: np.ndarray) -> np.ndarray:
    rect = np.zeros((4, 2), dtype="float32")

    s = pts.sum(axis=1)
    rect[0] = pts[np.argmin(s)]
    rect[2] = pts[np.argmax(s)]

    diff = np.diff(pts, axis=1)
    rect[1] = pts[np.argmin(diff)]
    rect[3] = pts[np.argmax(diff)]

    return rect


def warp_card(bgr: np.ndarray, out_size: tuple[int, int] = (744, 1040)) -> tuple[np.ndarray, dict[str, Any]]:
    h, w = bgr.shape[:2]

    scale = 800.0 / max(h, w)
    resized = cv2.resize(bgr, (int(w * scale), int(h * scale)))

    gray = cv2.cvtColor(resized, cv2.COLOR_BGR2GRAY)
    gray = cv2.GaussianBlur(gray, (5, 5), 0)
    edges = cv2.Canny(gray, 50, 150)

    contours, _ = cv2.findContours(edges, cv2.RETR_LIST, cv2.CHAIN_APPROX_SIMPLE)
    contours = sorted(contours, key=cv2.contourArea, reverse=True)[:25]

    quad = None
    for cnt in contours:
        peri = cv2.arcLength(cnt, True)
        approx = cv2.approxPolyDP(cnt, 0.02 * peri, True)
        if len(approx) == 4:
            quad = approx.reshape(4, 2)
            break

    debug: dict[str, Any] = {
        "found_quad": quad is not None,
    }

    if quad is None:
        rect = cv2.minAreaRect(max(contours, key=cv2.contourArea)) if contours else ((w / 2, h / 2), (w, h), 0)
        box = cv2.boxPoints(rect)
        quad = box.astype("float32")
        debug["fallback"] = "minAreaRect"

    quad = quad.astype("float32")
    quad = quad / scale

    rect = _order_points(quad)
    out_w, out_h = out_size
    dst = np.array(
        [
            [0, 0],
            [out_w - 1, 0],
            [out_w - 1, out_h - 1],
            [0, out_h - 1],
        ],
        dtype="float32",
    )

    m = cv2.getPerspectiveTransform(rect, dst)
    warped = cv2.warpPerspective(bgr, m, (out_w, out_h))

    debug["out_w"] = out_w
    debug["out_h"] = out_h

    return warped, debug
