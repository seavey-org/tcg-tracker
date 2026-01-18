#!/usr/bin/env python3
"""Download sample card images from all Pokemon sets for testing.

Usage:
    python scripts/download_test_images.py --out-dir /tmp/test_images --sets-file path/to/sets.json
"""

import argparse
import json
import sys
import time
from pathlib import Path
from urllib.request import urlopen, Request
from urllib.error import HTTPError, URLError


def download_image(url: str, dest: Path, timeout: int = 15) -> bool:
    """Download an image from URL to destination path."""
    try:
        req = Request(url, headers={"User-Agent": "Pokemon-TCG-Tester/1.0"})
        with urlopen(req, timeout=timeout) as response:
            data = response.read()
            dest.parent.mkdir(parents=True, exist_ok=True)
            with open(dest, "wb") as f:
                f.write(data)
            return True
    except (HTTPError, URLError, TimeoutError) as e:
        print(f"  Failed to download {url}: {e}")
        return False


def get_card_image_url(set_id: str, card_num: str, hires: bool = True) -> str:
    """Get Pokemon TCG image URL for a card."""
    suffix = "_hires" if hires else ""
    return f"https://images.pokemontcg.io/{set_id}/{card_num}{suffix}.png"


def main() -> int:
    parser = argparse.ArgumentParser(description="Download test images from all Pokemon sets")
    parser.add_argument(
        "--out-dir",
        default="/tmp/pokemon_test_images",
        help="Output directory for downloaded images",
    )
    parser.add_argument(
        "--sets-file",
        default="backend/data/pokemon-tcg-data-master/sets/en.json",
        help="Path to sets JSON file",
    )
    parser.add_argument(
        "--cards-per-set",
        type=int,
        default=1,
        help="Number of cards to download per set",
    )
    parser.add_argument(
        "--skip-existing",
        action="store_true",
        help="Skip sets that already have images",
    )
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    # Load sets
    sets_path = Path(args.sets_file)
    if not sets_path.exists():
        # Try relative to script location
        sets_path = Path(__file__).parent.parent.parent / args.sets_file

    if not sets_path.exists():
        print(f"Sets file not found: {args.sets_file}")
        return 1

    with open(sets_path) as f:
        sets = json.load(f)

    print(f"Found {len(sets)} sets")
    print(f"Output directory: {out_dir}")
    print()

    downloaded = 0
    failed = 0
    skipped = 0

    for i, s in enumerate(sets):
        set_id = s["id"]
        set_name = s["name"]
        series = s.get("series", "Unknown")

        # Check if we already have this set
        existing = list(out_dir.glob(f"{set_id}_*.png"))
        if args.skip_existing and existing:
            skipped += 1
            continue

        print(f"[{i+1}/{len(sets)}] {set_id}: {set_name} ({series})")

        # Try to download a few card numbers (some sets have gaps)
        success = False
        for card_num in ["1", "2", "3", "4", "5", "10", "15", "20"]:
            url = get_card_image_url(set_id, card_num, hires=True)
            dest = out_dir / f"{set_id}_{card_num}.png"

            if download_image(url, dest):
                print(f"  Downloaded card #{card_num}")
                downloaded += 1
                success = True
                break

            # Rate limiting
            time.sleep(0.1)

        if not success:
            # Try low-res
            for card_num in ["1", "2", "3"]:
                url = get_card_image_url(set_id, card_num, hires=False)
                dest = out_dir / f"{set_id}_{card_num}.png"

                if download_image(url, dest):
                    print(f"  Downloaded card #{card_num} (low-res)")
                    downloaded += 1
                    success = True
                    break

        if not success:
            print("  FAILED - no images available")
            failed += 1

        # Rate limiting
        time.sleep(0.2)

    print()
    print("Summary:")
    print(f"  Downloaded: {downloaded}")
    print(f"  Failed: {failed}")
    print(f"  Skipped: {skipped}")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
