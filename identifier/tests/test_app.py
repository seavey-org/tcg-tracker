"""Tests for FastAPI app endpoints."""

import base64
import io

import pytest
from PIL import Image

# Skip entire module if FastAPI is not installed
fastapi = pytest.importorskip("fastapi")
from fastapi.testclient import TestClient  # noqa: E402


class TestHealthEndpoint:
    """Tests for the /health endpoint."""

    def test_health_returns_ok(self) -> None:
        """Test that health endpoint returns ok status."""
        from identifier.app import app

        client = TestClient(app)
        response = client.get("/health")

        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"
        assert data["ocr_engine"] == "easyocr"
        assert "ocr_engine_ready" in data
        assert "gpu_available" in data


class TestOCREndpoint:
    """Tests for the /ocr endpoint."""

    @pytest.fixture
    def client(self):
        """Create test client."""
        from identifier.app import app
        return TestClient(app)

    @pytest.fixture
    def sample_image_b64(self) -> str:
        """Create a sample image with some text-like features."""
        # Create a simple test image with high contrast text area
        img = Image.new("RGB", (400, 300), color=(255, 255, 255))
        pixels = img.load()
        
        # Add some dark text-like pixels
        for y in range(100, 130):
            for x in range(50, 200):
                if (x + y) % 10 < 5:  # Create striped pattern
                    pixels[x, y] = (0, 0, 0)

        buf = io.BytesIO()
        img.save(buf, format="JPEG")
        return base64.b64encode(buf.getvalue()).decode("ascii")

    def test_ocr_requires_image(self, client) -> None:
        """Test that image_b64 parameter is required."""
        response = client.post("/ocr", json={})
        assert response.status_code == 422

    def test_ocr_rejects_invalid_base64(self, client) -> None:
        """Test that invalid base64 data is rejected."""
        response = client.post("/ocr", json={"image_b64": "not-valid-base64!!!"})
        assert response.status_code == 400
        assert "base64" in response.json()["detail"].lower()

    def test_ocr_rejects_invalid_image(self, client) -> None:
        """Test that invalid image data is rejected."""
        # Valid base64, but not an image
        invalid_b64 = base64.b64encode(b"not an image").decode("ascii")
        response = client.post("/ocr", json={"image_b64": invalid_b64})
        assert response.status_code == 400
        assert "decode" in response.json()["detail"].lower()

    def test_ocr_returns_expected_fields(self, client, sample_image_b64) -> None:
        """Test that OCR returns all expected fields."""
        response = client.post("/ocr", json={"image_b64": sample_image_b64})
        
        assert response.status_code == 200
        data = response.json()
        
        assert "text" in data
        assert "lines" in data
        assert "confidence" in data
        assert "auto_rotated" in data
        assert "original_size" in data
        assert "processed_size" in data
        assert "elapsed_ms" in data

    def test_ocr_respects_auto_rotate_false(self, client, sample_image_b64) -> None:
        """Test that auto_rotate=false is respected."""
        response = client.post(
            "/ocr",
            json={"image_b64": sample_image_b64, "auto_rotate": False}
        )
        
        assert response.status_code == 200
        data = response.json()
        assert data["auto_rotated"] is False

    def test_ocr_respects_max_dimension(self, client) -> None:
        """Test that max_dimension parameter affects processing."""
        # Create a larger image
        img = Image.new("RGB", (2000, 1500), color=(255, 255, 255))
        buf = io.BytesIO()
        img.save(buf, format="JPEG")
        large_image_b64 = base64.b64encode(buf.getvalue()).decode("ascii")
        
        response = client.post(
            "/ocr",
            json={"image_b64": large_image_b64, "max_dimension": 800}
        )
        
        assert response.status_code == 200
        data = response.json()
        
        # Original should be 2000x1500
        assert data["original_size"] == [1500, 2000]
        
        # Processed should be scaled down to max 800
        processed = data["processed_size"]
        assert max(processed) <= 800
