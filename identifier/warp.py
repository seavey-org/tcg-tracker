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
    out_w, out_h = out_size

    debug: dict[str, Any] = {
        "found_quad": False,
        "input_size": (w, h),
        "out_w": out_w,
        "out_h": out_h,
    }

    # Check if image is already card-shaped (no perspective correction needed)
    # Card aspect ratio is approximately 2.5:3.5 = 0.714
    aspect = w / h
    target_aspect = 2.5 / 3.5  # ~0.714

    # If aspect ratio is close to card ratio, just resize (clean TCG images)
    if 0.65 <= aspect <= 0.80:
        debug["fallback"] = "direct-resize"
        debug["reason"] = f"aspect={aspect:.3f} close to card ratio"
        warped = cv2.resize(bgr, (out_w, out_h), interpolation=cv2.INTER_LANCZOS4)
        return warped, debug

    # For images that need perspective correction (photos with background)
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
            # Check if the quad covers a reasonable portion of the image
            quad_area = cv2.contourArea(approx)
            image_area = resized.shape[0] * resized.shape[1]
            if quad_area > 0.1 * image_area:  # Must cover at least 10% of image
                quad = approx.reshape(4, 2)
                debug["found_quad"] = True
                debug["quad_area_ratio"] = quad_area / image_area
                break

    if quad is None:
        if contours:
            rect = cv2.minAreaRect(max(contours, key=cv2.contourArea))
            box = cv2.boxPoints(rect)
            quad = box.astype("float32")
            debug["fallback"] = "minAreaRect"
        else:
            # No contours found - use image corners
            debug["fallback"] = "no-contours"
            warped = cv2.resize(bgr, (out_w, out_h), interpolation=cv2.INTER_LANCZOS4)
            return warped, debug

    quad = quad.astype("float32")
    quad = quad / scale

    rect = _order_points(quad)
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

    return warped, debug
