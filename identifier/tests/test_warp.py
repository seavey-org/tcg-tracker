"""Tests for card warping functionality."""

import numpy as np
import pytest

from identifier.warp import warp_card, _order_points


class TestOrderPoints:
    """Tests for _order_points helper function."""

    def test_order_points_reorders_corners(self) -> None:
        """Test that points are ordered: top-left, top-right, bottom-right, bottom-left."""
        # Points in random order
        pts = np.array([[100, 100], [0, 0], [100, 0], [0, 100]], dtype="float32")

        ordered = _order_points(pts)

        # Should be: TL, TR, BR, BL
        assert tuple(ordered[0]) == (0.0, 0.0)      # Top-left
        assert tuple(ordered[1]) == (100.0, 0.0)    # Top-right
        assert tuple(ordered[2]) == (100.0, 100.0)  # Bottom-right
        assert tuple(ordered[3]) == (0.0, 100.0)    # Bottom-left

    def test_order_points_handles_rotated_rect(self) -> None:
        """Test ordering for a slightly rotated rectangle."""
        pts = np.array([[10, 5], [90, 10], [85, 95], [5, 90]], dtype="float32")

        ordered = _order_points(pts)

        # Should still have 4 points
        assert ordered.shape == (4, 2)
        # Sum of top-left should be smallest
        assert ordered[0].sum() == min(pts.sum(axis=1))


class TestWarpCard:
    """Tests for warp_card function."""

    def test_warp_card_default_output_size(self) -> None:
        """Test that default output size is 744x1040."""
        # Create a simple test image with a card-like rectangle
        img = np.zeros((500, 400, 3), dtype=np.uint8)
        # Draw a white rectangle
        img[50:450, 50:350] = 255

        warped, debug = warp_card(img)

        assert warped.shape == (1040, 744, 3)
        assert debug["out_w"] == 744
        assert debug["out_h"] == 1040

    def test_warp_card_custom_output_size(self) -> None:
        """Test custom output dimensions."""
        img = np.zeros((500, 400, 3), dtype=np.uint8)
        img[50:450, 50:350] = 255

        warped, debug = warp_card(img, out_size=(300, 400))

        assert warped.shape == (400, 300, 3)

    def test_warp_card_finds_quad(self) -> None:
        """Test that a clear card outline produces found_quad=True."""
        img = np.zeros((600, 500, 3), dtype=np.uint8)
        # Draw a clear white rectangle with black border
        img[100:500, 100:400] = 255

        _, debug = warp_card(img)

        # With a clear rectangle, should find a quad
        assert debug["found_quad"] is True

    def test_warp_card_fallback_on_no_quad(self) -> None:
        """Test fallback behavior when no quadrilateral is found."""
        # Create an image with no clear edges
        img = np.random.randint(100, 150, (500, 400, 3), dtype=np.uint8)

        warped, debug = warp_card(img)

        # Should still produce output (fallback to minAreaRect)
        assert warped.shape[0] > 0
        assert warped.shape[1] > 0

    def test_warp_card_handles_small_image(self) -> None:
        """Test handling of small input images."""
        img = np.zeros((100, 80, 3), dtype=np.uint8)
        img[10:90, 10:70] = 200

        warped, debug = warp_card(img)

        # Should still produce standard output size
        assert warped.shape == (1040, 744, 3)

    def test_warp_card_handles_large_image(self) -> None:
        """Test handling of large input images."""
        img = np.zeros((4000, 3000, 3), dtype=np.uint8)
        img[200:3800, 200:2800] = 180

        warped, debug = warp_card(img)

        assert warped.shape == (1040, 744, 3)

    def test_warp_card_preserves_content(self) -> None:
        """Test that warping preserves image content (not all black/white)."""
        img = np.zeros((600, 450, 3), dtype=np.uint8)
        # Create a gradient pattern inside the "card"
        for i in range(100, 500):
            for j in range(75, 375):
                img[i, j] = [(i - 100) % 256, (j - 75) % 256, 128]

        warped, _ = warp_card(img)

        # Warped image should have varied content
        assert warped.std() > 10  # Not uniform
