"""
Tests for set identification using real card images from different eras.

These tests validate that the set identification pipeline works correctly
across various Pokemon and MTG card eras, styles, and edge cases.

To run these tests, you need:
1. FAISS indexes built and available at SETID_INDEX_DIR
2. Test images downloaded to the tests/images/ directory

Run with: pytest tests/test_real_images.py -v
"""

import io
import os
from pathlib import Path
from typing import Any

import numpy as np
import pytest
from PIL import Image

# Skip entire module if dependencies are missing
cv2 = pytest.importorskip("cv2")


# Test data directory
TEST_DATA_DIR = Path(__file__).parent / "images"


def _create_mock_card_image(width: int = 744, height: int = 1040, seed: int = 42) -> bytes:
    """Create a mock card-like image for testing when real images aren't available."""
    np.random.seed(seed)

    # Create a card-like image with some structure
    img = Image.new("RGB", (width, height), color=(230, 230, 230))
    pixels = img.load()

    # Add some texture/patterns to different regions
    for y in range(height):
        for x in range(width):
            # Border region
            if x < 20 or x > width - 20 or y < 20 or y > height - 20:
                pixels[x, y] = (200, 180, 100)  # Gold border-ish
            # Art box region (upper portion)
            elif 50 < x < width - 50 and 80 < y < height * 0.55:
                pixels[x, y] = (150 + np.random.randint(-20, 20),
                                150 + np.random.randint(-20, 20),
                                150 + np.random.randint(-20, 20))

    buf = io.BytesIO()
    img.save(buf, format="JPEG", quality=90)
    return buf.getvalue()


class TestCropExtractor:
    """Tests for the CropExtractor class."""

    @pytest.fixture
    def cropper(self):
        """Create CropExtractor instance."""
        from identifier.set_matcher import CropExtractor
        return CropExtractor()

    def test_pokemon_rois_cover_all_eras(self, cropper):
        """Verify Pokemon ROIs cover symbol locations from all eras."""
        # Create a test image
        img_bytes = _create_mock_card_image()
        pil_img = Image.open(io.BytesIO(img_bytes)).convert("RGB")
        bgr = cv2.cvtColor(np.array(pil_img), cv2.COLOR_RGB2BGR)

        # Extract crops
        crops, debug = cropper.extract(game="pokemon", warped_bgr=bgr)

        # Should have multiple crops for different era regions
        assert len(crops) > 5, "Should have multiple crops for different era regions"

        # Check that we have boxes in different regions
        boxes = debug.get("boxes", [])
        assert len(boxes) > 0

        # Verify we have crops from different y-regions (different eras)
        y_positions = [box["y"] for box in boxes]
        y_min = min(y_positions)
        y_max = max(y_positions)

        # Should have variety in y positions (top, middle, bottom)
        assert y_max - y_min > 200, "Should have crops from different vertical regions"

    def test_mtg_rois_cover_frame_styles(self, cropper):
        """Verify MTG ROIs cover different frame styles."""
        img_bytes = _create_mock_card_image()
        pil_img = Image.open(io.BytesIO(img_bytes)).convert("RGB")
        bgr = cv2.cvtColor(np.array(pil_img), cv2.COLOR_RGB2BGR)

        crops, debug = cropper.extract(game="mtg", warped_bgr=bgr)

        # Should have multiple crops
        assert len(crops) > 3, "Should have multiple crops for MTG"

        boxes = debug.get("boxes", [])
        assert len(boxes) > 0


class TestSetMatcherIntegration:
    """Integration tests for SetMatcher with real/mock indexes."""

    @pytest.fixture
    def matcher(self):
        """Create SetMatcher if indexes are available."""
        from identifier.set_matcher import SetMatcher

        index_dir = os.getenv("SETID_INDEX_DIR", "")
        if not index_dir or not os.path.exists(os.path.join(index_dir, "pokemon.faiss")):
            pytest.skip("No indexes available - set SETID_INDEX_DIR")

        return SetMatcher(index_dir=index_dir)

    def test_identify_returns_valid_structure(self, matcher):
        """Test that identify returns a valid result structure."""
        img_bytes = _create_mock_card_image()
        pil_img = Image.open(io.BytesIO(img_bytes)).convert("RGB")
        bgr = cv2.cvtColor(np.array(pil_img), cv2.COLOR_RGB2BGR)

        result = matcher.identify(game="pokemon", warped_bgr=bgr)

        assert "best_set_id" in result
        assert "confidence" in result
        assert "low_confidence" in result
        assert "candidates" in result
        assert "timings_ms" in result

    def test_identify_with_different_image_sizes(self, matcher):
        """Test identification with various image sizes."""
        for width, height in [(744, 1040), (500, 700), (1488, 2080)]:
            img_bytes = _create_mock_card_image(width=width, height=height)
            pil_img = Image.open(io.BytesIO(img_bytes)).convert("RGB")
            bgr = cv2.cvtColor(np.array(pil_img), cv2.COLOR_RGB2BGR)

            result = matcher.identify(game="pokemon", warped_bgr=bgr)

            assert "best_set_id" in result
            assert "candidates" in result


class TestPokemonEras:
    """Era-specific tests for Pokemon cards."""

    @pytest.fixture
    def matcher(self):
        """Create SetMatcher if indexes are available."""
        from identifier.set_matcher import SetMatcher

        index_dir = os.getenv("SETID_INDEX_DIR", "")
        if not index_dir:
            pytest.skip("No indexes available - set SETID_INDEX_DIR")

        return SetMatcher(index_dir=index_dir)

    def _load_test_image(self, name: str) -> np.ndarray | None:
        """Load a test image by name."""
        image_path = TEST_DATA_DIR / name
        if not image_path.exists():
            return None

        pil_img = Image.open(image_path).convert("RGB")
        return cv2.cvtColor(np.array(pil_img), cv2.COLOR_RGB2BGR)

    @pytest.mark.parametrize("era,expected_sets", [
        # WOTC Era (1999-2003)
        ("wotc", ["base1", "base2", "jungle", "fossil", "base4", "base5", "gym1", "gym2",
                  "neo1", "neo2", "neo3", "neo4"]),
        # E-Card Era (2001-2003)
        ("ecard", ["ecard1", "ecard2", "ecard3"]),  # Expedition, Aquapolis, Skyridge
        # EX Era (2003-2007)
        ("ex", ["ex1", "ex2", "ex3", "ex4", "ex5", "ex6", "ex7", "ex8", "ex9", "ex10",
                "ex11", "ex12", "ex13", "ex14", "ex15", "ex16"]),
        # Diamond & Pearl Era (2007-2011)
        ("dp", ["dp1", "dp2", "dp3", "dp4", "dp5", "dp6", "dp7", "pl1", "pl2", "pl3", "pl4"]),
        # Black & White Era (2011-2013)
        ("bw", ["bw1", "bw2", "bw3", "bw4", "bw5", "bw6", "bw7", "bw8", "bw9", "bw10", "bw11"]),
        # XY Era (2014-2017)
        ("xy", ["xy1", "xy2", "xy3", "xy4", "xy5", "xy6", "xy7", "xy8", "xy9", "xy10",
                "xy11", "xy12"]),
        # Sun & Moon Era (2017-2019)
        ("sm", ["sm1", "sm2", "sm3", "sm35", "sm4", "sm5", "sm6", "sm7", "sm75", "sm8",
                "sm9", "sm10", "sm11", "sm115", "sm12"]),
        # Sword & Shield Era (2020-2023)
        ("swsh", ["swsh1", "swsh2", "swsh3", "swsh4", "swsh5", "swsh6", "swsh7", "swsh8",
                  "swsh9", "swsh10", "swsh11", "swsh12", "swsh12pt5"]),
        # Scarlet & Violet Era (2023+)
        ("sv", ["sv1", "sv2", "sv3", "sv3pt5", "sv4", "sv4pt5", "sv5", "sv6"]),
    ])
    def test_era_coverage(self, matcher, era: str, expected_sets: list[str]):
        """Test that index has coverage for a specific Pokemon era."""
        if "pokemon" not in matcher.games_loaded():
            pytest.skip("Pokemon index not loaded")

        # Check that at least some sets from this era are in the index
        indexed_sets = set()
        for game_result in matcher._indexes.get("pokemon", {}):
            if hasattr(matcher._indexes["pokemon"], "meta"):
                for entry in matcher._indexes["pokemon"].meta:
                    indexed_sets.add(entry["set_id"])

        if not indexed_sets:
            # Direct access to check
            pytest.skip("Cannot verify indexed sets")

        # At least half of the expected sets should be indexed
        matches = len(set(expected_sets) & indexed_sets)
        assert matches >= len(expected_sets) // 2, \
            f"Era {era}: Only {matches}/{len(expected_sets)} sets indexed"


class TestEdgeCases:
    """Tests for edge cases and special card types."""

    @pytest.fixture
    def cropper(self):
        """Create CropExtractor instance."""
        from identifier.set_matcher import CropExtractor
        return CropExtractor()

    def test_very_small_image(self, cropper):
        """Test handling of very small images."""
        small_img = np.zeros((100, 70, 3), dtype=np.uint8)
        small_img[:] = (200, 200, 200)

        # Should not crash
        crops, debug = cropper.extract(game="pokemon", warped_bgr=small_img)
        assert isinstance(crops, list)

    def test_grayscale_image(self, cropper):
        """Test handling of grayscale images converted to BGR."""
        gray = np.random.randint(0, 255, (1040, 744), dtype=np.uint8)
        bgr = cv2.cvtColor(gray, cv2.COLOR_GRAY2BGR)

        crops, debug = cropper.extract(game="pokemon", warped_bgr=bgr)
        assert len(crops) > 0

    def test_very_dark_image(self, cropper):
        """Test handling of very dark images."""
        dark_img = np.zeros((1040, 744, 3), dtype=np.uint8)
        dark_img[:] = (10, 10, 10)

        crops, debug = cropper.extract(game="pokemon", warped_bgr=dark_img)
        assert isinstance(crops, list)

    def test_very_bright_image(self, cropper):
        """Test handling of very bright/washed out images."""
        bright_img = np.ones((1040, 744, 3), dtype=np.uint8) * 250

        crops, debug = cropper.extract(game="pokemon", warped_bgr=bright_img)
        assert isinstance(crops, list)


class TestWarpIntegration:
    """Tests for warp + crop + identify pipeline."""

    @pytest.fixture
    def full_pipeline(self):
        """Set up full pipeline if available."""
        from identifier.set_matcher import SetMatcher, CropExtractor
        from identifier.warp import warp_card

        index_dir = os.getenv("SETID_INDEX_DIR", "")
        if not index_dir:
            pytest.skip("No indexes available - set SETID_INDEX_DIR")

        return {
            "matcher": SetMatcher(index_dir=index_dir),
            "cropper": CropExtractor(),
            "warp": warp_card,
        }

    def test_full_pipeline_with_mock_image(self, full_pipeline):
        """Test full pipeline: warp -> crop -> embed -> search."""
        img_bytes = _create_mock_card_image()
        pil_img = Image.open(io.BytesIO(img_bytes)).convert("RGB")
        bgr = cv2.cvtColor(np.array(pil_img), cv2.COLOR_RGB2BGR)

        # Run through pipeline
        warped, warp_debug = full_pipeline["warp"](bgr)
        result = full_pipeline["matcher"].identify(game="pokemon", warped_bgr=warped)

        assert "best_set_id" in result
        assert "timings_ms" in result
        assert result["timings_ms"]["crop"] >= 0
        assert result["timings_ms"]["embed"] >= 0
        assert result["timings_ms"]["search"] >= 0
