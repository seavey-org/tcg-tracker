#!/usr/bin/env python3
"""
Tests for set identification accuracy using real card images.

These tests download actual card images from Pokemon TCG API and verify
that the set identification correctly identifies the set.

Test data: Each entry has:
- set_id: The expected set ID
- card_num: The card number in that set
- era: The Pokemon era (for debugging)
"""

import io
import os
from pathlib import Path
from typing import Any
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

import numpy as np
import pytest
from PIL import Image

cv2 = pytest.importorskip("cv2")

# Test cases: (set_id, card_number, era, description)
# These are cards from different eras with distinctive set symbols
POKEMON_TEST_CASES = [
    # Base Set era - WOTC (1999-2000)
    ("base1", "4", "wotc", "Charizard Base Set"),
    ("base1", "58", "wotc", "Pikachu Base Set"),
    ("base2", "10", "wotc", "Scyther Jungle"),
    ("base3", "5", "wotc", "Gengar Fossil"),

    # Neo era - WOTC (2000-2002)
    ("neo1", "9", "neo", "Lugia Neo Genesis"),
    ("neo2", "13", "neo", "Espeon Neo Discovery"),
    ("neo3", "2", "neo", "Entei Neo Revelation"),

    # e-Card era (2002-2003)
    ("ecard1", "6", "ecard", "Charizard Expedition"),
    ("ecard2", "3", "ecard", "Entei Aquapolis"),
    ("ecard3", "3", "ecard", "Charizard Skyridge"),

    # EX era (2003-2007)
    ("ex1", "100", "ex", "Charizard ex Ruby & Sapphire"),
    ("ex5", "15", "ex", "Charizard EX Team Rocket Returns"),
    ("ex12", "4", "ex", "Charizard Delta Species"),

    # Diamond & Pearl era (2007-2010)
    ("dp1", "3", "dp", "Dialga Diamond & Pearl"),
    ("pl3", "6", "dp", "Charizard G Supreme Victors"),

    # Black & White era (2011-2013)
    ("bw1", "20", "bw", "Reshiram Black & White"),
    ("bw6", "136", "bw", "Charizard Boundaries Crossed"),

    # XY era (2014-2017)
    ("xy1", "12", "xy", "Charizard EX XY Base"),
    ("xy2", "11", "xy", "Charizard Flashfire"),
    ("xy12", "12", "xy", "Charizard Evolutions"),

    # Sun & Moon era (2017-2019)
    ("sm1", "18", "sm", "Incineroar Sun & Moon"),
    ("sm3", "20", "sm", "Charizard GX Burning Shadows"),
    ("sm10", "14", "sm", "Reshiram & Charizard GX Unbroken Bonds"),

    # Sword & Shield era (2020-2023)
    ("swsh1", "25", "swsh", "Charizard V Sword & Shield"),
    ("swsh4", "44", "swsh", "Pikachu VMAX Vivid Voltage"),
    ("swsh9", "TG03", "swsh", "Charizard VSTAR Brilliant Stars TG"),
    ("swsh11", "174", "swsh", "Charizard V Lost Origin"),

    # Scarlet & Violet era (2023+)
    ("sv1", "169", "sv", "Koraidon ex Scarlet & Violet"),
    ("sv2", "99", "sv", "Charizard ex Paldea Evolved"),
    ("sv3pt5", "6", "sv", "Charizard ex 151"),
    ("sv4", "54", "sv", "Charizard Paradox Rift"),
]

# MTG test cases
MTG_TEST_CASES = [
    # Would need to add MTG test cases with Scryfall image URLs
]


def get_pokemon_image_url(set_id: str, card_num: str, hires: bool = True) -> str:
    """Get the Pokemon TCG API image URL."""
    suffix = "_hires" if hires else ""
    return f"https://images.pokemontcg.io/{set_id}/{card_num}{suffix}.png"


def download_image(url: str, cache_dir: Path | None = None) -> bytes | None:
    """Download an image, optionally caching it."""
    # Check cache first
    if cache_dir:
        cache_key = url.replace("/", "_").replace(":", "_")
        cache_path = cache_dir / cache_key
        if cache_path.exists():
            return cache_path.read_bytes()

    try:
        req = Request(url, headers={"User-Agent": "Pokemon-TCG-Tester/1.0"})
        with urlopen(req, timeout=15) as response:
            data = response.read()

            # Cache the result
            if cache_dir:
                cache_dir.mkdir(parents=True, exist_ok=True)
                cache_path.write_bytes(data)

            return data
    except (HTTPError, URLError, TimeoutError) as e:
        print(f"Failed to download {url}: {e}")
        return None


def image_to_bgr(image_bytes: bytes) -> np.ndarray:
    """Convert image bytes to OpenCV BGR format."""
    pil_img = Image.open(io.BytesIO(image_bytes)).convert("RGB")
    rgb = np.array(pil_img)
    return cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR)


class TestSetIdentificationAccuracy:
    """Test set identification accuracy on real card images."""

    @pytest.fixture(scope="class")
    def matcher(self):
        """Create SetMatcher if indexes are available."""
        from identifier.set_matcher import SetMatcher

        index_dir = os.getenv("SETID_INDEX_DIR", "")
        if not index_dir:
            pytest.skip("No indexes available - set SETID_INDEX_DIR")

        faiss_path = os.path.join(index_dir, "pokemon.faiss")
        if not os.path.exists(faiss_path):
            pytest.skip(f"Pokemon index not found at {faiss_path}")

        return SetMatcher(index_dir=index_dir)

    @pytest.fixture(scope="class")
    def cache_dir(self, tmp_path_factory):
        """Persistent cache directory for downloaded images."""
        return tmp_path_factory.mktemp("card_images")

    @pytest.fixture(scope="class")
    def warp_fn(self):
        """Get the warp function."""
        from identifier.warp import warp_card
        return warp_card

    @pytest.mark.parametrize("set_id,card_num,era,description", POKEMON_TEST_CASES)
    def test_pokemon_card_identification(
        self, matcher, cache_dir, warp_fn, set_id: str, card_num: str, era: str, description: str
    ):
        """Test that a specific Pokemon card is correctly identified."""
        # Try high-res first, then low-res
        url = get_pokemon_image_url(set_id, card_num, hires=True)
        img_bytes = download_image(url, cache_dir)

        if img_bytes is None:
            url = get_pokemon_image_url(set_id, card_num, hires=False)
            img_bytes = download_image(url, cache_dir)

        if img_bytes is None:
            pytest.skip(f"Could not download image for {set_id}/{card_num}")

        # Convert to BGR
        bgr = image_to_bgr(img_bytes)

        # Warp the card
        warped, warp_debug = warp_fn(bgr)

        # Identify
        result = matcher.identify(game="pokemon", warped_bgr=warped)

        # Check results
        best_set_id = result["best_set_id"]
        confidence = result["confidence"]
        candidates = result["candidates"]
        low_conf = result["low_confidence"]

        # Debug info
        print(f"\n{description} ({set_id}/{card_num}):")
        print(f"  Expected: {set_id}")
        print(f"  Got: {best_set_id} (confidence={confidence:.3f}, low_conf={low_conf})")
        print(f"  Warp: {warp_debug}")
        if candidates[:5]:
            print(f"  Top candidates: {[(c['set_id'], f\"{c['score']:.3f}\") for c in candidates[:5]]}")

        # Check if correct set is in top candidates (more lenient for now)
        candidate_set_ids = [c["set_id"] for c in candidates[:10]]

        # Either exact match or in top 10 candidates
        assert best_set_id == set_id or set_id in candidate_set_ids, \
            f"Expected {set_id} but got {best_set_id}. Not in top 10: {candidate_set_ids}"


class TestSetIdentificationStats:
    """Run all tests and report overall accuracy statistics."""

    @pytest.fixture(scope="class")
    def matcher(self):
        """Create SetMatcher if indexes are available."""
        from identifier.set_matcher import SetMatcher

        index_dir = os.getenv("SETID_INDEX_DIR", "")
        if not index_dir:
            pytest.skip("No indexes available - set SETID_INDEX_DIR")

        return SetMatcher(index_dir=index_dir)

    @pytest.fixture(scope="class")
    def warp_fn(self):
        """Get the warp function."""
        from identifier.warp import warp_card
        return warp_card

    def test_overall_accuracy(self, matcher, warp_fn, tmp_path):
        """Test overall accuracy across all Pokemon test cases."""
        cache_dir = tmp_path / "images"

        results: dict[str, dict[str, Any]] = {}
        correct_top1 = 0
        correct_top5 = 0
        correct_top10 = 0
        total = 0
        skipped = 0

        for set_id, card_num, era, description in POKEMON_TEST_CASES:
            url = get_pokemon_image_url(set_id, card_num, hires=True)
            img_bytes = download_image(url, cache_dir)

            if img_bytes is None:
                url = get_pokemon_image_url(set_id, card_num, hires=False)
                img_bytes = download_image(url, cache_dir)

            if img_bytes is None:
                skipped += 1
                continue

            bgr = image_to_bgr(img_bytes)
            warped, warp_debug = warp_fn(bgr)
            result = matcher.identify(game="pokemon", warped_bgr=warped)

            best = result["best_set_id"]
            candidates = [c["set_id"] for c in result["candidates"][:10]]

            total += 1

            if best == set_id:
                correct_top1 += 1
            if set_id in candidates[:5]:
                correct_top5 += 1
            if set_id in candidates[:10]:
                correct_top10 += 1

            results[f"{set_id}/{card_num}"] = {
                "expected": set_id,
                "got": best,
                "correct_top1": best == set_id,
                "correct_top5": set_id in candidates[:5],
                "era": era,
            }

        # Print summary
        print("\n" + "=" * 60)
        print("SET IDENTIFICATION ACCURACY REPORT")
        print("=" * 60)
        print(f"\nTotal cards tested: {total}")
        print(f"Skipped (download failed): {skipped}")
        print(f"\nAccuracy:")
        print(f"  Top-1 (exact match): {correct_top1}/{total} ({100*correct_top1/total:.1f}%)")
        print(f"  Top-5 (in top 5): {correct_top5}/{total} ({100*correct_top5/total:.1f}%)")
        print(f"  Top-10 (in top 10): {correct_top10}/{total} ({100*correct_top10/total:.1f}%)")

        # Per-era breakdown
        print("\nPer-era accuracy (Top-1):")
        era_stats: dict[str, tuple[int, int]] = {}
        for key, data in results.items():
            era = data["era"]
            if era not in era_stats:
                era_stats[era] = (0, 0)
            correct, total_era = era_stats[era]
            era_stats[era] = (correct + (1 if data["correct_top1"] else 0), total_era + 1)

        for era, (correct, total_era) in sorted(era_stats.items()):
            pct = 100 * correct / total_era if total_era > 0 else 0
            print(f"  {era}: {correct}/{total_era} ({pct:.1f}%)")

        # List failures
        failures = [k for k, v in results.items() if not v["correct_top5"]]
        if failures:
            print(f"\nCards NOT in top-5:")
            for k in failures:
                v = results[k]
                print(f"  {k}: expected={v['expected']}, got={v['got']}")

        # Assert minimum accuracy threshold
        min_top5_accuracy = 0.6  # 60% should be in top 5
        actual_top5_accuracy = correct_top5 / total if total > 0 else 0
        assert actual_top5_accuracy >= min_top5_accuracy, \
            f"Top-5 accuracy {actual_top5_accuracy:.1%} is below threshold {min_top5_accuracy:.1%}"
