#!/usr/bin/env python3
"""Test script for the identifier service.

Usage:
    python scripts/test_service.py [--url URL] [--image PATH] [--game GAME]

If no image is provided, a synthetic test image is generated.
"""

import argparse
import io
import sys
import time
from pathlib import Path

import requests
from PIL import Image


def create_test_image(game: str = "pokemon") -> bytes:
    """Create a synthetic card image for testing."""
    # Standard card dimensions (2.5" x 3.5" at ~300 DPI ≈ 744x1040)
    img = Image.new("RGB", (744, 1040), color=(240, 240, 230))

    # Add some content in the set icon region
    pixels = img.load()
    if game == "pokemon":
        # Pokemon set icon is typically in the lower right
        for y in range(870, 960):
            for x in range(580, 680):
                # Create a simple geometric shape
                if (x - 630) ** 2 + (y - 915) ** 2 < 35**2:
                    pixels[x, y] = (200, 160, 50)  # Gold/amber
    else:
        # MTG set icon is in the middle-right area
        for y in range(440, 520):
            for x in range(620, 700):
                if (x - 660) ** 2 + (y - 480) ** 2 < 30**2:
                    pixels[x, y] = (180, 80, 40)  # Brownish

    buf = io.BytesIO()
    img.save(buf, format="JPEG", quality=90)
    return buf.getvalue()


def load_image(path: str) -> bytes:
    """Load an image file."""
    with open(path, "rb") as f:
        return f.read()


def test_health(base_url: str) -> dict:
    """Test the health endpoint."""
    print(f"Testing health endpoint at {base_url}/health ...")
    try:
        resp = requests.get(f"{base_url}/health", timeout=10)
        resp.raise_for_status()
        data = resp.json()
        print(f"  Status: {data.get('status')}")
        print(f"  Index dir: {data.get('index_dir')}")
        print(f"  Games loaded: {data.get('games_loaded', [])}")
        return data
    except requests.RequestException as e:
        print(f"  ERROR: {e}")
        return {}


def test_identify(base_url: str, image: bytes, game: str) -> dict:
    """Test the identify-set endpoint."""
    print(f"\nTesting identify-set for {game} ...")
    start = time.time()

    try:
        resp = requests.post(
            f"{base_url}/identify-set",
            files={"image": ("card.jpg", image, "image/jpeg")},
            data={"game": game},
            timeout=60,
        )
        elapsed = time.time() - start

        if resp.status_code != 200:
            print(f"  ERROR: HTTP {resp.status_code}")
            print(f"  Response: {resp.text[:500]}")
            return {}

        data = resp.json()
        print(f"  Best set ID: {data.get('best_set_id', 'N/A')}")
        print(f"  Confidence: {data.get('confidence', 0):.3f}")
        print(f"  Low confidence: {data.get('low_confidence', False)}")

        candidates = data.get("candidates", [])[:5]
        if candidates:
            print("  Top candidates:")
            for c in candidates:
                print(f"    - {c['set_id']}: {c['score']:.3f}")

        timings = data.get("timings_ms", {})
        if timings:
            print(f"  Timings: {timings}")

        print(f"  Total request time: {elapsed*1000:.0f}ms")
        return data

    except requests.RequestException as e:
        print(f"  ERROR: {e}")
        return {}


def main() -> int:
    parser = argparse.ArgumentParser(description="Test the set identifier service")
    parser.add_argument(
        "--url",
        default="http://127.0.0.1:8099",
        help="Base URL of the identifier service",
    )
    parser.add_argument(
        "--image",
        help="Path to a card image to test with",
    )
    parser.add_argument(
        "--game",
        choices=["pokemon", "mtg"],
        default="pokemon",
        help="Game type for testing",
    )
    args = parser.parse_args()

    print("=" * 60)
    print("Set Identifier Service Test")
    print("=" * 60)

    # Health check
    health = test_health(args.url)
    if not health:
        print("\nService appears to be down or unreachable.")
        return 1

    games_loaded = health.get("games_loaded", [])
    if not games_loaded:
        print("\nWARNING: No games/indexes are loaded.")
        print("The service is running but cannot identify sets without indexes.")
        print("Run the index builders to create indexes.")
        return 1

    # Load or create test image
    if args.image:
        print(f"\nUsing image: {args.image}")
        image = load_image(args.image)
    else:
        print(f"\nUsing synthetic test image for {args.game}")
        image = create_test_image(args.game)

    # Test identification
    result = test_identify(args.url, image, args.game)
    if not result:
        print("\nIdentification failed.")
        return 1

    print("\n" + "=" * 60)
    print("Test completed successfully!")
    return 0


if __name__ == "__main__":
    sys.exit(main())
