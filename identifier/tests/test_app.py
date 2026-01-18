"""Tests for FastAPI app endpoints."""

import io
import os
import sys

import numpy as np
import pytest
from PIL import Image

# Skip entire module if FastAPI is not installed
fastapi = pytest.importorskip("fastapi")
from fastapi.testclient import TestClient


class TestHealthEndpoint:
    """Tests for the /health endpoint."""

    def test_health_returns_ok(self) -> None:
        """Test that health endpoint returns ok status."""
        # Import here to avoid import errors if deps missing
        from identifier.app import app

        client = TestClient(app)
        response = client.get("/health")

        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"
        assert "games_loaded" in data


class TestIdentifySetEndpoint:
    """Tests for the /identify-set endpoint."""

    @pytest.fixture
    def client(self):
        """Create test client."""
        from identifier.app import app

        return TestClient(app)

    @pytest.fixture
    def sample_image_bytes(self) -> bytes:
        """Create a sample card-like image for testing."""
        # Create a simple test image
        img = Image.new("RGB", (744, 1040), color=(200, 200, 200))
        # Add some features in the set icon region
        pixels = img.load()
        for y in range(850, 950):
            for x in range(550, 650):
                pixels[x, y] = (100, 50, 50)

        buf = io.BytesIO()
        img.save(buf, format="JPEG")
        return buf.getvalue()

    def test_identify_set_requires_game(self, client, sample_image_bytes) -> None:
        """Test that game parameter is required."""
        response = client.post(
            "/identify-set",
            files={"image": ("card.jpg", sample_image_bytes, "image/jpeg")},
        )

        # Should fail with 422 (missing required field)
        assert response.status_code == 422

    def test_identify_set_rejects_invalid_game(
        self, client, sample_image_bytes
    ) -> None:
        """Test that invalid game values are rejected."""
        response = client.post(
            "/identify-set",
            files={"image": ("card.jpg", sample_image_bytes, "image/jpeg")},
            data={"game": "yugioh"},
        )

        assert response.status_code == 400
        assert "game must be" in response.json()["detail"]

    def test_identify_set_rejects_empty_image(self, client) -> None:
        """Test that empty images are rejected."""
        response = client.post(
            "/identify-set",
            files={"image": ("card.jpg", b"", "image/jpeg")},
            data={"game": "pokemon"},
        )

        assert response.status_code == 400
        assert "empty image" in response.json()["detail"]

    def test_identify_set_rejects_invalid_image(self, client) -> None:
        """Test that invalid image data is rejected."""
        response = client.post(
            "/identify-set",
            files={"image": ("card.jpg", b"not an image", "image/jpeg")},
            data={"game": "pokemon"},
        )

        assert response.status_code == 400
        assert "decode" in response.json()["detail"].lower()


class TestIdentifySetWithIndexes:
    """Tests that require loaded indexes - skipped if indexes unavailable."""

    @pytest.fixture
    def client_with_indexes(self):
        """Create test client with index loading check."""
        from identifier.app import app, matcher

        if not matcher.games_loaded():
            pytest.skip("No indexes loaded - skipping integration tests")

        return TestClient(app)

    @pytest.fixture
    def pokemon_card_image(self) -> bytes:
        """Create a more realistic Pokemon card mock image."""
        img = Image.new("RGB", (744, 1040), color=(255, 255, 240))

        # Simulate set icon region with distinct color
        pixels = img.load()
        for y in range(880, 970):
            for x in range(600, 690):
                pixels[x, y] = (220, 180, 50)  # Goldish color

        buf = io.BytesIO()
        img.save(buf, format="JPEG", quality=95)
        return buf.getvalue()

    def test_identify_set_returns_candidates(
        self, client_with_indexes, pokemon_card_image
    ) -> None:
        """Test that identify-set returns candidates when indexes are loaded."""
        response = client_with_indexes.post(
            "/identify-set",
            files={"image": ("card.jpg", pokemon_card_image, "image/jpeg")},
            data={"game": "pokemon"},
        )

        assert response.status_code == 200
        data = response.json()
        assert "best_set_id" in data
        assert "confidence" in data
        assert "low_confidence" in data
        assert "candidates" in data
        assert "timings_ms" in data

    def test_identify_set_returns_timings(
        self, client_with_indexes, pokemon_card_image
    ) -> None:
        """Test that timing information is included."""
        response = client_with_indexes.post(
            "/identify-set",
            files={"image": ("card.jpg", pokemon_card_image, "image/jpeg")},
            data={"game": "pokemon"},
        )

        data = response.json()
        timings = data["timings_ms"]
        assert "total" in timings
        assert timings["total"] >= 0
