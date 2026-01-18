#!/usr/bin/env python3
"""
Fetch MTG card data from Scryfall API bulk data.

This script downloads the 'Default Cards' bulk JSON from Scryfall,
which contains ~75k unique cards with oracle_id and image_uris.

Usage:
    python -m identifier.scripts.data_fetchers.fetch_mtg_scryfall
    python -m identifier.scripts.data_fetchers.fetch_mtg_scryfall --output ./data/mtg_cards.json
    python -m identifier.scripts.data_fetchers.fetch_mtg_scryfall --to-db

Environment variables:
    DATABASE_URL - SQLite connection string (default: sqlite:///./tcg_scanner.db)
"""

import argparse
import json
import logging
import sys
import time
from datetime import datetime
from pathlib import Path
from typing import Any
from urllib.request import Request, urlopen

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

SCRYFALL_BULK_DATA_URL = "https://api.scryfall.com/bulk-data"
USER_AGENT = "TCGScanner/1.0 (https://github.com/codyseavey/tcg-tracker)"


def get_bulk_data_url(data_type: str = "default_cards") -> str:
    """Get the download URL for bulk data from Scryfall."""
    logger.info("Fetching bulk data catalog from Scryfall...")
    req = Request(SCRYFALL_BULK_DATA_URL, headers={"User-Agent": USER_AGENT})

    with urlopen(req, timeout=30) as response:
        bulk_data = json.loads(response.read().decode("utf-8"))

    for item in bulk_data.get("data", []):
        if item.get("type") == data_type:
            download_uri = item.get("download_uri")
            size_mb = item.get("size", 0) / (1024 * 1024)
            logger.info(f"Found {data_type} bulk data: {size_mb:.1f} MB")
            return download_uri

    raise ValueError(f"Bulk data type '{data_type}' not found")


def download_bulk_data(url: str) -> list[dict]:
    """Download and parse bulk JSON data from Scryfall."""
    logger.info(f"Downloading bulk data from: {url}")
    req = Request(url, headers={"User-Agent": USER_AGENT})

    start_time = time.time()
    with urlopen(req, timeout=600) as response:
        raw_data = response.read()

    elapsed = time.time() - start_time
    logger.info(f"Downloaded {len(raw_data) / (1024 * 1024):.1f} MB in {elapsed:.1f}s")

    # Scryfall serves JSON directly (not gzipped)
    logger.info("Parsing JSON data...")
    cards = json.loads(raw_data.decode("utf-8"))
    logger.info(f"Parsed {len(cards)} cards")

    return cards


def extract_card_data(card: dict) -> dict[str, Any]:
    """Extract relevant fields from a Scryfall card object."""
    # Get best image URL (prefer large, fallback to normal, then small)
    image_uris = card.get("image_uris", {})
    if not image_uris and card.get("card_faces"):
        # Double-faced cards have images on faces
        image_uris = card["card_faces"][0].get("image_uris", {})

    image_large = image_uris.get("large") or image_uris.get("normal")
    image_small = image_uris.get("small")

    # Build card ID: set_code + collector_number
    set_code = card.get("set", "").lower()
    collector_number = card.get("collector_number", "")
    card_id = f"mtg-{set_code}-{collector_number}"

    return {
        "card_id": card_id,
        "name": card.get("name", ""),
        "game": "mtg",
        "set_id": set_code,
        "collector_number": collector_number,
        "rarity": card.get("rarity", ""),

        # MTG specific
        "oracle_id": card.get("oracle_id"),
        "mana_cost": card.get("mana_cost"),
        "cmc": card.get("cmc"),
        "type_line": card.get("type_line"),
        "oracle_text": card.get("oracle_text"),
        "power": card.get("power"),
        "toughness": card.get("toughness"),
        "artist": card.get("artist"),

        # Images
        "image_small": image_small,
        "image_large": image_large,

        # Determine if it's a foil variant
        "is_holo": card.get("foil", False) and not card.get("nonfoil", True),
    }


def extract_set_data(card: dict) -> dict[str, Any]:
    """Extract set information from a Scryfall card object."""
    return {
        "set_id": card.get("set", "").lower(),
        "name": card.get("set_name", ""),
        "game": "mtg",
        "scryfall_id": card.get("set_id"),
        "release_date": card.get("released_at"),
    }


def fetch_and_save_to_json(output_path: str) -> int:
    """Fetch MTG data and save to JSON file."""
    url = get_bulk_data_url("default_cards")
    cards = download_bulk_data(url)

    # Process cards
    processed_cards = []
    sets_seen = {}

    for card in cards:
        try:
            card_data = extract_card_data(card)
            processed_cards.append(card_data)

            # Track sets
            set_id = card_data["set_id"]
            if set_id and set_id not in sets_seen:
                sets_seen[set_id] = extract_set_data(card)
        except Exception as e:
            logger.warning(f"Error processing card {card.get('name', 'unknown')}: {e}")

    output = {
        "fetched_at": datetime.utcnow().isoformat(),
        "source": "scryfall",
        "total_cards": len(processed_cards),
        "total_sets": len(sets_seen),
        "sets": list(sets_seen.values()),
        "cards": processed_cards,
    }

    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)

    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    logger.info(f"Saved {len(processed_cards)} cards and {len(sets_seen)} sets to {output_path}")
    return len(processed_cards)


def fetch_and_save_to_db() -> int:
    """Fetch MTG data and save to database."""
    from identifier.database import init_db, SessionLocal
    from identifier.database.repository import CardRepository, CardSetRepository

    # Initialize database
    init_db()

    url = get_bulk_data_url("default_cards")
    cards = download_bulk_data(url)

    db = SessionLocal()
    try:
        set_repo = CardSetRepository(db)
        card_repo = CardRepository(db)

        sets_seen = {}
        cards_processed = 0

        for i, card in enumerate(cards):
            try:
                # Extract and upsert set if not seen
                card_data = extract_card_data(card)
                set_id = card_data["set_id"]

                if set_id and set_id not in sets_seen:
                    set_data = extract_set_data(card)
                    set_repo.upsert(set_data)
                    sets_seen[set_id] = True

                # Upsert card
                card_repo.upsert(card_data)
                cards_processed += 1

                # Log progress every 10000 cards
                if (i + 1) % 10000 == 0:
                    logger.info(f"Processed {i + 1} / {len(cards)} cards...")
                    db.commit()  # Commit in batches

            except Exception as e:
                logger.warning(f"Error processing card {card.get('name', 'unknown')}: {e}")

        db.commit()
        logger.info(f"Saved {cards_processed} cards and {len(sets_seen)} sets to database")
        return cards_processed

    finally:
        db.close()


def main():
    parser = argparse.ArgumentParser(
        description="Fetch MTG card data from Scryfall bulk API"
    )
    parser.add_argument(
        "--output",
        type=str,
        default="./data/mtg_cards.json",
        help="Output JSON file path (default: ./data/mtg_cards.json)",
    )
    parser.add_argument(
        "--to-db",
        action="store_true",
        help="Save directly to database instead of JSON file",
    )

    args = parser.parse_args()

    try:
        if args.to_db:
            count = fetch_and_save_to_db()
        else:
            count = fetch_and_save_to_json(args.output)

        logger.info(f"Successfully fetched {count} MTG cards from Scryfall")
        return 0

    except Exception as e:
        logger.error(f"Failed to fetch MTG data: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
