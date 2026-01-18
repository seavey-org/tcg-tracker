#!/usr/bin/env python3
"""Analyze crop debug output to verify set symbols are being captured.

This script examines the debug output from debug_crops.py and reports on:
- Which sets have crops in expected regions
- Potential issues with specific eras
"""

import argparse
import json
import sys
from pathlib import Path


def analyze_crops(debug_dir: Path) -> dict:
    """Analyze crop debug output."""
    results = {
        "total": 0,
        "with_wotc_region": 0,
        "with_modern_region": 0,
        "by_era": {},
    }

    # Era classification based on set ID prefixes
    era_prefixes = {
        "base": "Base/WOTC",
        "gym": "Gym",
        "neo": "Neo",
        "ecard": "E-Card",
        "ex": "EX",
        "np": "Nintendo Promo",
        "pop": "POP",
        "dp": "Diamond & Pearl",
        "pl": "Platinum",
        "hgss": "HeartGold & SoulSilver",
        "hsp": "HGSS Promo",
        "bw": "Black & White",
        "xy": "XY",
        "sm": "Sun & Moon",
        "swsh": "Sword & Shield",
        "sv": "Scarlet & Violet",
        "rsv": "Scarlet & Violet",
        "zsv": "Scarlet & Violet",
        "me": "Mega Evolution",
    }

    for card_dir in sorted(debug_dir.iterdir()):
        if not card_dir.is_dir():
            continue

        debug_file = card_dir / "debug.txt"
        if not debug_file.exists():
            continue

        results["total"] += 1

        # Read debug info
        with open(debug_file) as f:
            content = f.read()

        # Parse boxes
        try:
            boxes_line = [l for l in content.split("\n") if l.startswith("boxes=")][0]
            boxes_str = boxes_line.replace("boxes=", "")
            boxes = eval(boxes_str)  # Safe since we generated this
        except (IndexError, SyntaxError):
            boxes = []

        # Classify set by era
        set_id = card_dir.name.rsplit("_", 1)[0]
        era = "Other"
        for prefix, era_name in era_prefixes.items():
            if set_id.startswith(prefix):
                era = era_name
                break

        if era not in results["by_era"]:
            results["by_era"][era] = {"total": 0, "sets": []}
        results["by_era"][era]["total"] += 1
        results["by_era"][era]["sets"].append(set_id)

        # Check if we have crops in WOTC region (middle-right: x > 70%, y 45-70%)
        has_wotc = False
        has_modern = False

        for box in boxes:
            x, y, w, h = box["x"], box["y"], box["w"], box["h"]
            # Assuming 744x1040 standard size
            x_pct = x / 744
            y_pct = y / 1040
            y_end_pct = (y + h) / 1040

            # WOTC region: x > 70%, y between 45-70%
            if x_pct > 0.70 and y_pct >= 0.40 and y_end_pct <= 0.75:
                has_wotc = True

            # Modern region: x < 40%, y > 85%
            if x_pct < 0.40 and y_pct > 0.82:
                has_modern = True

        if has_wotc:
            results["with_wotc_region"] += 1
        if has_modern:
            results["with_modern_region"] += 1

    return results


def main() -> int:
    parser = argparse.ArgumentParser(description="Analyze crop debug output")
    parser.add_argument(
        "--debug-dir",
        default="/tmp/pokemon_crops_debug",
        help="Directory with debug output from debug_crops.py",
    )
    args = parser.parse_args()

    debug_dir = Path(args.debug_dir)
    if not debug_dir.exists():
        print(f"Debug directory not found: {debug_dir}")
        return 1

    results = analyze_crops(debug_dir)

    print("=" * 60)
    print("Crop Analysis Results")
    print("=" * 60)
    print()
    print(f"Total sets analyzed: {results['total']}")
    print(f"Sets with WOTC region crops: {results['with_wotc_region']}")
    print(f"Sets with modern region crops: {results['with_modern_region']}")
    print()
    print("By Era:")
    print("-" * 40)

    for era, data in sorted(results["by_era"].items()):
        print(f"  {era}: {data['total']} sets")

    return 0


if __name__ == "__main__":
    sys.exit(main())
