"""Tests for CropExtractor class."""

import numpy as np
import pytest
from PIL import Image

from identifier.set_matcher import CropExtractor


class TestCropExtractor:
    """Tests for CropExtractor functionality."""

    def setup_method(self) -> None:
        self.extractor = CropExtractor()

    def test_extract_pokemon_returns_crops(self) -> None:
        """Test that extract returns crops for a Pokemon card image."""
        # Create a mock warped card image (744x1040 is standard output size)
        img = np.zeros((1040, 744, 3), dtype=np.uint8)
        # Add contrast in WOTC-era middle-right region (80-96% x, 52-66% y)
        img[540:686, 595:714] = 255
        # Add contrast in modern bottom-left region (1-15% x, 89-97% y)
        img[925:1009, 7:111] = 200

        crops, debug = self.extractor.extract(game="pokemon", warped_bgr=img)

        assert len(crops) > 0
        assert isinstance(crops[0], Image.Image)
        assert "boxes" in debug
        assert len(debug["boxes"]) > 0

    def test_extract_mtg_returns_crops(self) -> None:
        """Test that extract returns crops for an MTG card image."""
        img = np.zeros((1040, 744, 3), dtype=np.uint8)
        # Add some contrast in the middle right where MTG set symbol would be
        img[400:550, 550:700] = 255

        crops, debug = self.extractor.extract(game="mtg", warped_bgr=img)

        assert len(crops) > 0
        assert isinstance(crops[0], Image.Image)
        assert "boxes" in debug

    def test_extract_has_max_crop_limit(self) -> None:
        """Test that we don't return too many crops (performance)."""
        img = np.zeros((1040, 744, 3), dtype=np.uint8)
        # Create many high-contrast regions in the ROI areas
        # WOTC region (middle-right)
        for y in range(520, 700, 40):
            for x in range(560, 740, 40):
                img[y : y + 30, x : x + 30] = 255
        # Modern region (bottom-left)
        for y in range(900, 1020, 40):
            for x in range(10, 250, 40):
                img[y : y + 30, x : x + 30] = 200

        crops, debug = self.extractor.extract(game="pokemon", warped_bgr=img)

        # Should be limited to max_crops (10)
        assert len(crops) <= 10

    def test_extract_boxes_have_valid_coords(self) -> None:
        """Test that debug boxes have valid coordinate information."""
        img = np.zeros((1040, 744, 3), dtype=np.uint8)
        # Add contrast in WOTC region
        img[540:680, 590:710] = 128

        _, debug = self.extractor.extract(game="pokemon", warped_bgr=img)

        for box in debug["boxes"]:
            assert "x" in box and box["x"] >= 0
            assert "y" in box and box["y"] >= 0
            assert "w" in box and box["w"] > 0
            assert "h" in box and box["h"] > 0

    def test_contour_proposals_filters_by_size(self) -> None:
        """Test that contour proposals filter out very small/large blobs."""
        # Create ROI with both small and appropriately sized regions
        roi = np.zeros((200, 200, 3), dtype=np.uint8)
        # Very small blob (should be filtered)
        roi[10:15, 10:15] = 255
        # Appropriately sized blob (should be kept)
        roi[50:100, 50:100] = 255

        proposals = self.extractor._contour_proposals(roi, (0, 0, 200, 200))

        # Should find the larger region
        assert len(proposals) >= 1
        # All proposals should be in valid format
        for box, crop in proposals:
            assert len(box) == 4
            assert isinstance(crop, np.ndarray)

    def test_contour_proposals_filters_by_aspect_ratio(self) -> None:
        """Test that very elongated shapes are filtered out."""
        roi = np.zeros((200, 200, 3), dtype=np.uint8)
        # Very wide shape (aspect > 2.2)
        roi[50:60, 10:190] = 255

        proposals = self.extractor._contour_proposals(roi, (0, 0, 200, 200))

        # Should filter out the elongated shape
        assert len(proposals) == 0

    def test_empty_image_returns_crops(self) -> None:
        """Test that even a blank image returns some ROI crops."""
        img = np.zeros((1040, 744, 3), dtype=np.uint8)

        crops, debug = self.extractor.extract(game="pokemon", warped_bgr=img)

        # Should still return the predefined ROI crops even if no contours found
        assert len(crops) > 0
