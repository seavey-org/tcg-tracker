import base64
import io
import os
import time
from typing import Any

import cv2
import numpy as np
from fastapi import FastAPI, File, Form, HTTPException, Query, UploadFile
from fastapi.responses import JSONResponse
from PIL import Image

from identifier.set_matcher import SetMatcher
from identifier.warp import warp_card


APP_HOST = os.getenv("HOST", "127.0.0.1")
APP_PORT = int(os.getenv("PORT", "8099"))

SETID_INDEX_DIR = os.getenv("SETID_INDEX_DIR", "")
SET_ID_DEBUG = os.getenv("SET_ID_DEBUG", "0") == "1"


app = FastAPI(title="tcg-identifier")

matcher = SetMatcher(index_dir=SETID_INDEX_DIR)


@app.get("/health")
def health() -> dict[str, Any]:
    return {
        "status": "ok",
        "index_dir": SETID_INDEX_DIR,
        "games_loaded": matcher.games_loaded(),
        "device": str(matcher.device),
        "cuda_available": matcher.cuda_available,
        "cuda_device_name": matcher.cuda_device_name,
    }


@app.post("/identify-set")
def identify_set(
    image: UploadFile = File(...),
    game: str = Form(...),
    debug: bool = Query(False),
) -> JSONResponse:
    if game not in ("pokemon", "mtg"):
        raise HTTPException(status_code=400, detail="game must be 'pokemon' or 'mtg'")

    raw = image.file.read()
    if not raw:
        raise HTTPException(status_code=400, detail="empty image")

    start = time.time()

    np_img = np.frombuffer(raw, dtype=np.uint8)
    bgr = cv2.imdecode(np_img, cv2.IMREAD_COLOR)
    if bgr is None:
        raise HTTPException(status_code=400, detail="failed to decode image")

    warped, warp_debug = warp_card(bgr)

    result = matcher.identify(game=game, warped_bgr=warped)

    resp: dict[str, Any] = {
        "best_set_id": result["best_set_id"],
        "confidence": result["confidence"],
        "low_confidence": result["low_confidence"],
        "candidates": result["candidates"],
        "timings_ms": {
            "total": int((time.time() - start) * 1000),
            **result.get("timings_ms", {}),
        },
    }

    if debug and SET_ID_DEBUG:
        debug_payload: dict[str, Any] = {
            "warp": warp_debug,
            "crops": result.get("debug", {}).get("crops"),
        }

        warped_rgb = cv2.cvtColor(warped, cv2.COLOR_BGR2RGB)
        pil = Image.fromarray(warped_rgb)
        buf = io.BytesIO()
        pil.save(buf, format="JPEG", quality=90)
        debug_payload["warped_image_b64"] = base64.b64encode(buf.getvalue()).decode("ascii")

        resp["debug"] = debug_payload

    return JSONResponse(resp)
