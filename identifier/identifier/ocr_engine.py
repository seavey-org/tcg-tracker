"""
GPU-accelerated OCR engine using EasyOCR.

The engine is initialized once and cached for reuse across all requests.
This avoids the 2-5 second model loading time on each request.

Supports multi-language OCR for international card scanning:
- English (en) - default
- German (de) - "KP" for HP, German Pokemon names
- French (fr) - "PV" for HP, French Pokemon names  
- Italian (it) - "PS" for HP, Italian Pokemon names
- Japanese (ja) - kanji/hiragana/katakana

Usage:
    from identifier.ocr_engine import get_ocr_engine, downscale_image

    # Get cached engine (initializes on first call)
    engine = get_ocr_engine(use_gpu=True)
    
    # Optionally downscale large images
    image = downscale_image(image, max_dim=1280)
    
    # Read text with automatic rotation detection
    text = engine.read_text(image, rotation_info=[90, 180, 270])
"""

import logging
from typing import Any, cast

import cv2
import numpy as np

logger = logging.getLogger(__name__)

# Default languages for multi-language card scanning
# Japanese (ja) can only be combined with English due to EasyOCR constraints.
# This default supports scanning both English and Japanese cards.
# For European languages only, set OCR_LANGUAGES="en,de,fr,it" (no Japanese)
DEFAULT_LANGUAGES = ["ja", "en"]

# Module-level cached engine singleton
_engine: "EasyOCREngine | None" = None
_engine_use_gpu: bool | None = None
_engine_languages: list[str] | None = None


class EasyOCREngine:
    """EasyOCR wrapper with rotation support and caching."""

    def __init__(self, use_gpu: bool = True, languages: list[str] | None = None):
        """Initialize EasyOCR engine.

        Args:
            use_gpu: Whether to use GPU acceleration (highly recommended)
            languages: List of language codes (default: DEFAULT_LANGUAGES)
                      Supports: en, de, fr, it, ja for card scanning

        Raises:
            RuntimeError: If EasyOCR cannot be initialized
        """
        try:
            import easyocr

            self._languages = languages or DEFAULT_LANGUAGES
            self._use_gpu = use_gpu
            self._reader = easyocr.Reader(
                self._languages,
                gpu=use_gpu,
                verbose=False,
            )
            logger.info(f"EasyOCR initialized (GPU={use_gpu}, languages={self._languages})")
        except ImportError:
            raise RuntimeError(
                "EasyOCR not installed. Install with: pip install easyocr"
            )
        except Exception as e:
            raise RuntimeError(f"Failed to initialize EasyOCR: {e}")

    @property
    def use_gpu(self) -> bool:
        """Whether GPU is being used."""
        return self._use_gpu

    def read_text(
        self,
        image: np.ndarray,
        rotation_info: list[int] | None = None,
    ) -> str:
        """Read text from image with optional rotation detection.

        Args:
            image: Input image (BGR or grayscale numpy array)
            rotation_info: List of rotation angles to try (e.g., [90, 180, 270]).
                          EasyOCR will try each rotation and return the best result.
                          Set to None to disable rotation detection.

        Returns:
            Extracted text as a single string with newline-separated lines.
        """
        # Performance-tuned parameters for card scanning:
        # - decoder='greedy': ~30% faster than default 'beamsearch'
        # - paragraph=False: Skip paragraph merging (not needed for cards)
        # - text_threshold=0.5: More permissive to catch small/faded text at card bottom
        #   (set codes, copyright, card numbers are small but critical)
        # - low_text=0.3: Better handling of separated characters in small text
        # NOTE: Intentionally NOT setting min_size - small text at card bottom is crucial
        result = self._reader.readtext(
            image,
            rotation_info=rotation_info,
            detail=0,
            decoder='greedy',
            paragraph=False,
            text_threshold=0.5,
            low_text=0.3,
        )
        texts = cast(list[str], result)
        lines = [text.strip() for text in texts if text and text.strip()]
        return "\n".join(lines)

    def read_text_with_boxes(
        self,
        image: np.ndarray,
        rotation_info: list[int] | None = None,
    ) -> list[dict[str, Any]]:
        """Read text with bounding boxes and confidence scores.

        Args:
            image: Input image (BGR or grayscale numpy array)
            rotation_info: List of rotation angles to try

        Returns:
            List of dicts with 'text', 'box', and 'confidence' keys.
            Box is [x_min, y_min, x_max, y_max].
        """
        # Same performance tuning as read_text
        # NOTE: No min_size filter - small text at card bottom is crucial for set detection
        result = self._reader.readtext(
            image,
            rotation_info=rotation_info,
            decoder='greedy',
            paragraph=False,
            text_threshold=0.5,
            low_text=0.3,
        )

        results = []
        for box, text, confidence in result:
            if text:
                # Convert polygon to bounding box
                xs = [p[0] for p in box]
                ys = [p[1] for p in box]
                bbox = [min(xs), min(ys), max(xs), max(ys)]

                results.append({
                    "text": text,
                    "box": bbox,
                    "confidence": float(confidence),
                })

        return results


def get_ocr_engine(
    use_gpu: bool = True, 
    languages: list[str] | None = None
) -> EasyOCREngine:
    """Get the cached OCR engine instance.

    The engine is initialized on first call and reused for subsequent calls.
    This avoids the expensive model loading on each request.

    Args:
        use_gpu: Whether to use GPU acceleration. If this differs from the
                cached engine's setting, a new engine will be created.
        languages: List of language codes. If this differs from the cached
                  engine's languages, a new engine will be created.
                  Default: DEFAULT_LANGUAGES (en, de, fr, it, ja)

    Returns:
        Cached EasyOCREngine instance.

    Raises:
        RuntimeError: If EasyOCR cannot be initialized (e.g., GPU not available
                     when use_gpu=True, or missing dependencies).
    """
    global _engine, _engine_use_gpu, _engine_languages

    langs = languages or DEFAULT_LANGUAGES
    
    if _engine is None or _engine_use_gpu != use_gpu or _engine_languages != langs:
        logger.info(f"Creating new EasyOCR engine (GPU={use_gpu}, languages={langs})")
        _engine = EasyOCREngine(use_gpu=use_gpu, languages=langs)
        _engine_use_gpu = use_gpu
        _engine_languages = langs

    return _engine


def init_ocr_engine(
    use_gpu: bool = True, 
    languages: list[str] | None = None
) -> EasyOCREngine:
    """Eagerly initialize the OCR engine at application startup.

    Call this during application initialization to ensure the engine is ready
    before the first request. This moves the ~3-5 second initialization time
    to startup rather than the first request.

    Args:
        use_gpu: Whether to use GPU acceleration.
        languages: List of language codes. Default: DEFAULT_LANGUAGES

    Returns:
        The initialized EasyOCREngine instance.

    Raises:
        RuntimeError: If initialization fails. The application should fail
                     to start in this case.
    """
    langs = languages or DEFAULT_LANGUAGES
    logger.info(f"Eagerly initializing OCR engine at startup (languages={langs})...")
    engine = get_ocr_engine(use_gpu=use_gpu, languages=langs)
    logger.info("OCR engine ready")
    return engine


def downscale_image(image: np.ndarray, max_dim: int = 1280) -> np.ndarray:
    """Downscale image if larger than max dimension.

    Large images are resized to have their maximum dimension equal to max_dim,
    maintaining aspect ratio. This improves OCR performance without significantly
    affecting accuracy for card scanning.

    Args:
        image: Input image (BGR numpy array)
        max_dim: Maximum dimension (width or height) in pixels

    Returns:
        Downscaled image, or original if already smaller than max_dim.
    """
    h, w = image.shape[:2]
    if max(h, w) <= max_dim:
        return image

    scale = max_dim / max(h, w)
    new_w = int(w * scale)
    new_h = int(h * scale)

    return cv2.resize(image, (new_w, new_h), interpolation=cv2.INTER_AREA)


def get_gpu_info() -> dict[str, Any]:
    """Get GPU availability and device information.

    Returns:
        Dict with 'available', 'device_name', and 'device_count' keys.
    """
    try:
        import torch

        available = torch.cuda.is_available()
        device_name = torch.cuda.get_device_name(0) if available else None
        device_count = torch.cuda.device_count() if available else 0

        return {
            "available": available,
            "device_name": device_name,
            "device_count": device_count,
        }
    except ImportError:
        return {
            "available": False,
            "device_name": None,
            "device_count": 0,
            "error": "torch not installed",
        }
    except Exception as e:
        return {
            "available": False,
            "device_name": None,
            "device_count": 0,
            "error": str(e),
        }
