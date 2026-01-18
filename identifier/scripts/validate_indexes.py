#!/usr/bin/env python3
"""
Validate set identification indexes for completeness and quality.

This script checks that the FAISS indexes have adequate coverage of all
sets and can be used as a CI check or manual validation tool.
"""

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any


def load_pokemon_sets(repo_root: str) -> list[dict[str, Any]]:
    """Load Pokemon set data from the local data directory."""
    path = os.path.join(repo_root, "backend", "data", "pokemon-tcg-data-master", "sets", "en.json")
    if not os.path.exists(path):
        return []
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_pokemon_index(index_dir: str, repo_root: str, min_coverage: float = 0.95) -> dict[str, Any]:
    """Validate Pokemon set index coverage.

    Args:
        index_dir: Directory containing the FAISS indexes
        repo_root: Root of the repository
        min_coverage: Minimum acceptable coverage (0-1)

    Returns:
        Validation report dict
    """
    result = {
        "game": "pokemon",
        "valid": False,
        "errors": [],
        "warnings": [],
    }

    # Check index files exist
    faiss_path = os.path.join(index_dir, "pokemon.faiss")
    meta_path = os.path.join(index_dir, "pokemon_meta.json")

    if not os.path.exists(faiss_path):
        result["errors"].append(f"Missing index file: {faiss_path}")
        return result

    if not os.path.exists(meta_path):
        result["errors"].append(f"Missing metadata file: {meta_path}")
        return result

    # Load metadata
    with open(meta_path, "r", encoding="utf-8") as f:
        meta = json.load(f)

    indexed_set_ids = set(entry["set_id"] for entry in meta)
    result["indexed_sets"] = len(indexed_set_ids)
    result["total_vectors"] = len(meta)

    # Load expected sets
    expected_sets = load_pokemon_sets(repo_root)
    if not expected_sets:
        result["warnings"].append("Could not load expected sets - skipping coverage check")
        result["valid"] = True
        return result

    expected_set_ids = set(s["id"] for s in expected_sets)
    result["expected_sets"] = len(expected_set_ids)

    # Check coverage
    missing_sets = expected_set_ids - indexed_set_ids
    extra_sets = indexed_set_ids - expected_set_ids

    coverage = len(indexed_set_ids & expected_set_ids) / len(expected_set_ids) if expected_set_ids else 0
    result["coverage_pct"] = round(coverage * 100, 2)

    if missing_sets:
        result["missing_sets"] = sorted(missing_sets)
        if len(missing_sets) > 10:
            result["warnings"].append(f"Missing {len(missing_sets)} sets (showing first 10): {sorted(missing_sets)[:10]}")
        else:
            result["warnings"].append(f"Missing sets: {sorted(missing_sets)}")

    if extra_sets:
        result["extra_sets"] = sorted(extra_sets)
        result["warnings"].append(f"Extra sets in index (not in expected): {sorted(extra_sets)[:5]}")

    # Check vectors per set
    vectors_per_set: dict[str, int] = {}
    for entry in meta:
        set_id = entry["set_id"]
        vectors_per_set[set_id] = vectors_per_set.get(set_id, 0) + 1

    min_vectors = min(vectors_per_set.values()) if vectors_per_set else 0
    max_vectors = max(vectors_per_set.values()) if vectors_per_set else 0
    avg_vectors = sum(vectors_per_set.values()) / len(vectors_per_set) if vectors_per_set else 0

    result["vectors_per_set"] = {
        "min": min_vectors,
        "max": max_vectors,
        "avg": round(avg_vectors, 2),
    }

    # Low vector count warning
    low_vector_sets = [sid for sid, count in vectors_per_set.items() if count < 3]
    if low_vector_sets:
        result["warnings"].append(f"{len(low_vector_sets)} sets have fewer than 3 vectors")

    # Determine validity
    if coverage < min_coverage:
        result["errors"].append(f"Coverage {coverage * 100:.1f}% is below minimum {min_coverage * 100:.1f}%")
    else:
        result["valid"] = True

    return result


def validate_mtg_index(index_dir: str, min_coverage: float = 0.90) -> dict[str, Any]:
    """Validate MTG set index coverage.

    Args:
        index_dir: Directory containing the FAISS indexes
        min_coverage: Minimum acceptable coverage (0-1)

    Returns:
        Validation report dict
    """
    result = {
        "game": "mtg",
        "valid": False,
        "errors": [],
        "warnings": [],
    }

    # Check index files exist
    faiss_path = os.path.join(index_dir, "mtg.faiss")
    meta_path = os.path.join(index_dir, "mtg_meta.json")

    if not os.path.exists(faiss_path):
        result["errors"].append(f"Missing index file: {faiss_path}")
        return result

    if not os.path.exists(meta_path):
        result["errors"].append(f"Missing metadata file: {meta_path}")
        return result

    # Load metadata
    with open(meta_path, "r", encoding="utf-8") as f:
        meta = json.load(f)

    indexed_set_ids = set(entry["set_id"] for entry in meta)
    result["indexed_sets"] = len(indexed_set_ids)
    result["total_vectors"] = len(meta)

    # MTG has many sets, we just check basic stats
    # (fetching all sets from Scryfall would be slow)

    # Check vectors per set
    vectors_per_set: dict[str, int] = {}
    for entry in meta:
        set_id = entry["set_id"]
        vectors_per_set[set_id] = vectors_per_set.get(set_id, 0) + 1

    min_vectors = min(vectors_per_set.values()) if vectors_per_set else 0
    max_vectors = max(vectors_per_set.values()) if vectors_per_set else 0
    avg_vectors = sum(vectors_per_set.values()) / len(vectors_per_set) if vectors_per_set else 0

    result["vectors_per_set"] = {
        "min": min_vectors,
        "max": max_vectors,
        "avg": round(avg_vectors, 2),
    }

    # Low vector count warning
    low_vector_sets = [sid for sid, count in vectors_per_set.items() if count < 3]
    if low_vector_sets:
        result["warnings"].append(f"{len(low_vector_sets)} sets have fewer than 3 vectors")

    # Check for validation report
    report_path = os.path.join(index_dir, "mtg_validation_report.json")
    if os.path.exists(report_path):
        with open(report_path, "r", encoding="utf-8") as f:
            build_report = json.load(f)
            coverage = build_report.get("coverage_pct", 0) / 100
            if coverage < min_coverage:
                result["errors"].append(f"Coverage {coverage * 100:.1f}% is below minimum {min_coverage * 100:.1f}%")
            else:
                result["valid"] = True
            result["coverage_pct"] = build_report.get("coverage_pct", 0)
    else:
        result["warnings"].append("No validation report found - cannot verify coverage")
        # Without a report, assume valid if we have a reasonable number of sets
        if len(indexed_set_ids) >= 100:
            result["valid"] = True
        else:
            result["errors"].append(f"Only {len(indexed_set_ids)} sets indexed - expected at least 100")

    return result


def main() -> None:
    parser = argparse.ArgumentParser(description="Validate set identification indexes")
    parser.add_argument("--index-dir", required=True, help="Directory containing FAISS indexes")
    parser.add_argument("--repo-root", default=".", help="Repository root for loading expected sets")
    parser.add_argument("--pokemon-min-coverage", type=float, default=0.95, help="Minimum Pokemon coverage")
    parser.add_argument("--mtg-min-coverage", type=float, default=0.90, help="Minimum MTG coverage")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    parser.add_argument("--fail-on-warning", action="store_true", help="Fail if any warnings")
    args = parser.parse_args()

    results = {
        "index_dir": args.index_dir,
        "games": {},
        "overall_valid": True,
    }

    # Validate Pokemon
    pokemon_result = validate_pokemon_index(args.index_dir, args.repo_root, args.pokemon_min_coverage)
    results["games"]["pokemon"] = pokemon_result
    if not pokemon_result["valid"]:
        results["overall_valid"] = False

    # Validate MTG
    mtg_result = validate_mtg_index(args.index_dir, args.mtg_min_coverage)
    results["games"]["mtg"] = mtg_result
    if not mtg_result["valid"]:
        results["overall_valid"] = False

    # Check for warnings if strict mode
    if args.fail_on_warning:
        for game, game_result in results["games"].items():
            if game_result.get("warnings"):
                results["overall_valid"] = False

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        print("=" * 60)
        print("INDEX VALIDATION REPORT")
        print("=" * 60)
        print(f"Index directory: {args.index_dir}")
        print()

        for game, game_result in results["games"].items():
            print(f"--- {game.upper()} ---")
            print(f"  Valid:          {game_result['valid']}")
            print(f"  Indexed sets:   {game_result.get('indexed_sets', 'N/A')}")
            print(f"  Total vectors:  {game_result.get('total_vectors', 'N/A')}")
            if "coverage_pct" in game_result:
                print(f"  Coverage:       {game_result['coverage_pct']}%")
            if "vectors_per_set" in game_result:
                vps = game_result["vectors_per_set"]
                print(f"  Vectors/set:    min={vps['min']}, avg={vps['avg']}, max={vps['max']}")

            if game_result.get("errors"):
                print("  ERRORS:")
                for err in game_result["errors"]:
                    print(f"    - {err}")

            if game_result.get("warnings"):
                print("  Warnings:")
                for warn in game_result["warnings"]:
                    print(f"    - {warn}")

            print()

        print("=" * 60)
        print(f"OVERALL: {'PASS' if results['overall_valid'] else 'FAIL'}")
        print("=" * 60)

    sys.exit(0 if results["overall_valid"] else 1)


if __name__ == "__main__":
    main()
