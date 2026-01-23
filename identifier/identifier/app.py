"""
Minimal identifier service providing OCR text extraction via EasyOCR.

This service is called by the Go backend for server-side OCR when the mobile app
uploads card images. It uses EasyOCR with GPU acceleration for fast text extraction.

Supports multi-language OCR via OCR_LANGUAGES env var. Default: Japanese + English.
Note: EasyOCR has language compatibility constraints:
  - Japanese (ja) can ONLY be combined with English
  - Latin scripts (en, de, fr, it) can be combined freely but NOT with Japanese

Examples:
  OCR_LANGUAGES="ja,en"        # Japanese + English (default, for scanning both)
  OCR_LANGUAGES="en,de,fr,it"  # European languages only (no Japanese)
  OCR_LANGUAGES="en"           # English only

Endpoints:
- GET /health - Service health check with GPU status
- POST /ocr - Extract text from base64-encoded image
"""

import base64
import os
import time
from typing import Any

import cv2
import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from .ocr_engine import (
    init_ocr_engine,
    downscale_image,
    get_gpu_info,
    DEFAULT_LANGUAGES,
)


APP_HOST = os.getenv("HOST", "127.0.0.1")
APP_PORT = int(os.getenv("PORT", "8099"))
USE_GPU = os.getenv("USE_GPU", "1") == "1"

# Parse OCR_LANGUAGES from env var (comma-separated) or use defaults
# See module docstring for language compatibility constraints
_ocr_languages_env = os.getenv("OCR_LANGUAGES", "")
OCR_LANGUAGES = (
    [lang.strip() for lang in _ocr_languages_env.split(",") if lang.strip()]
    if _ocr_languages_env
    else DEFAULT_LANGUAGES
)

app = FastAPI(title="tcg-identifier")

# Initialize OCR engine eagerly at startup to avoid first-request latency
# This will fail fast if EasyOCR cannot be initialized (e.g., missing dependencies)
print(f"[app] Initializing OCR engine (languages={OCR_LANGUAGES})...")
ocr_engine = init_ocr_engine(use_gpu=USE_GPU, languages=OCR_LANGUAGES)
print(f"[app] OCR engine ready (GPU={ocr_engine.use_gpu}, languages={OCR_LANGUAGES})")


class OCRRequest(BaseModel):
    """Request model for OCR extraction."""
    image_b64: str
    auto_rotate: bool = True  # Use EasyOCR's rotation_info for auto-rotation
    max_dimension: int = 1280  # Downscale large images for performance


@app.get("/health")
def health() -> dict[str, Any]:
    """
    Health check endpoint.
    
    Returns service status, GPU information, and language configuration for monitoring.
    """
    gpu_info = get_gpu_info()
    return {
        "status": "ok",
        "ocr_engine": "easyocr",
        "ocr_engine_ready": ocr_engine is not None,
        "ocr_using_gpu": ocr_engine.use_gpu if ocr_engine else False,
        "ocr_languages": OCR_LANGUAGES,
        "gpu_available": gpu_info.get("available", False),
        "gpu_name": gpu_info.get("device_name"),
    }


@app.post("/ocr")
def ocr_image(request: OCRRequest) -> JSONResponse:
    """
    Extract text from an image using EasyOCR.

    The cached OCR engine is used for fast inference. Images are automatically
    downscaled to max_dimension for performance. If auto_rotate is enabled,
    EasyOCR's built-in rotation detection is used.
    
    Args:
        request: OCRRequest with base64-encoded image
        
    Returns:
        JSON with extracted text, lines, confidence, and timing info
    """
    start = time.time()

    # Decode image
    try:
        raw = base64.b64decode(request.image_b64)
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Invalid base64 data: {e}")
    
    np_img = np.frombuffer(raw, dtype=np.uint8)
    bgr = cv2.imdecode(np_img, cv2.IMREAD_COLOR)
    if bgr is None:
        raise HTTPException(status_code=400, detail="Failed to decode image")

    original_shape = bgr.shape[:2]

    # Downscale for performance
    bgr = downscale_image(bgr, max_dim=request.max_dimension)
    downscaled_shape = bgr.shape[:2]

    # Use EasyOCR's rotation_info for automatic rotation handling
    # This is more efficient than running OCR 4 times
    rotation_info = [90, 180, 270] if request.auto_rotate else None

    # Run OCR using the cached engine
    text = ocr_engine.read_text(bgr, rotation_info=rotation_info)
    lines = [line.strip() for line in text.split('\n') if line.strip()]

    # Get confidence from detailed results
    confidence = 0.0
    try:
        boxes = ocr_engine.read_text_with_boxes(bgr, rotation_info=rotation_info)
        if boxes:
            confidence = sum(b.get("confidence", 0.0) for b in boxes) / len(boxes)
    except Exception:
        pass

    elapsed_ms = int((time.time() - start) * 1000)

    return JSONResponse({
        "text": text,
        "lines": lines,
        "confidence": round(confidence, 3),
        "auto_rotated": request.auto_rotate,
        "original_size": list(original_shape),
        "processed_size": list(downscaled_shape),
        "elapsed_ms": elapsed_ms,
    })
