#!/usr/bin/env python3
"""
Fetch Pokemon card data from PokemonTCG.io API (V2).

This script iterates through all Pokemon sets and cards via the PokemonTCG.io API,
caching card data and images, while flagging 1st Edition vs Unlimited cards.

Usage:
    python -m identifier.scripts.data_fetchers.fetch_pokemon_tcgio
    python -m identifier.scripts.data_fetchers.fetch_pokemon_tcgio --output ./data/pokemon_cards.json
    python -m identifier.scripts.data_fetchers.fetch_pokemon_tcgio --to-db
    python -m identifier.scripts.data_fetchers.fetch_pokemon_tcgio --download-images ./data/pokemon_images

Environment variables:
    POKEMONTCG_API_KEY - Optional API key for higher rate limits (https://pokemontcg.io/)
    DATABASE_URL - SQLite connection string (default: sqlite:///./tcg_scanner.db)
"""

import argparse
import json
import logging
import os
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

POKEMONTCG_API_BASE = "https://api.pokemontcg.io/v2"
USER_AGENT = "TCGScanner/1.0 (https://github.com/codyseavey/tcg-tracker)"

# Sets that have 1st Edition printings (base through Neo era)
FIRST_EDITION_SETS = {
    "base1", "base2", "base3", "base4", "base5", "base6",  # Base set era
    "gym1", "gym2",  # Gym series
    "neo1", "neo2", "neo3", "neo4",  # Neo series
    "ecard1", "ecard2", "ecard3",  # e-Card series (also had 1st Ed)
}

# Rate limiting
REQUEST_DELAY = 0.5  # seconds between requests (without API key)
REQUEST_DELAY_WITH_KEY = 0.1  # seconds with API key


def make_api_request(url: str, api_key: str | None = None) -> dict:
    """Make a request to the PokemonTCG.io API with rate limiting."""
    headers = {"User-Agent": USER_AGENT}
    if api_key:
        headers["X-Api-Key"] = api_key

    req = Request(url, headers=headers)

    delay = REQUEST_DELAY_WITH_KEY if api_key else REQUEST_DELAY
    time.sleep(delay)

    try:
        with urlopen(req, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except HTTPError as e:
        if e.code == 429:
            logger.warning("Rate limited, waiting 60 seconds...")
            time.sleep(60)
            return make_api_request(url, api_key)  # Retry
        raise


def fetch_all_sets(api_key: str | None = None) -> list[dict]:
    """Fetch all Pokemon TCG sets."""
    logger.info("Fetching all Pokemon sets...")
    url = f"{POKEMONTCG_API_BASE}/sets?orderBy=releaseDate"
    response = make_api_request(url, api_key)
    sets = response.get("data", [])
    logger.info(f"Found {len(sets)} sets")
    return sets


def fetch_cards_for_set(set_id: str, api_key: str | None = None) -> list[dict]:
    """Fetch all cards for a specific set with pagination."""
    cards = []
    page = 1
    page_size = 250

    while True:
        url = f"{POKEMONTCG_API_BASE}/cards?q=set.id:{set_id}&page={page}&pageSize={page_size}"
        response = make_api_request(url, api_key)

        page_cards = response.get("data", [])
        if not page_cards:
            break

        cards.extend(page_cards)

        total_count = response.get("totalCount", 0)
        if len(cards) >= total_count:
            break

        page += 1

    return cards


def extract_set_data(pokemon_set: dict) -> dict[str, Any]:
    """Extract relevant fields from a PokemonTCG.io set object."""
    set_id = pokemon_set.get("id", "")
    return {
        "set_id": set_id,
        "name": pokemon_set.get("name", ""),
        "game": "pokemon",
        "series": pokemon_set.get("series", ""),
        "release_date": pokemon_set.get("releaseDate"),
        "total_cards": pokemon_set.get("total"),
        "ptcgo_code": pokemon_set.get("ptcgoCode"),
        "symbol_url": pokemon_set.get("images", {}).get("symbol"),
        "logo_url": pokemon_set.get("images", {}).get("logo"),
    }


def extract_card_data(card: dict) -> dict[str, Any]:
    """Extract relevant fields from a PokemonTCG.io card object."""
    set_id = card.get("set", {}).get("id", "")
    number = card.get("number", "")
    card_id = f"pokemon-{set_id}-{number}"

    # Determine edition flags
    is_first_edition = False
    is_unlimited = True
    is_shadowless = False

    # Check if set can have 1st edition
    if set_id.lower() in FIRST_EDITION_SETS:
        # Most cards in these sets are unlimited; 1st editions are separate products
        # The API doesn't distinguish, so we set flags based on set capability
        is_unlimited = True
        # Note: To properly identify 1st edition, you need OCR on the physical card

    # Check for shadowless (only applies to base set)
    if set_id.lower() == "base1":
        # Shadowless detection requires OCR/image analysis
        pass

    # Extract types
    types = card.get("types", [])
    subtypes = card.get("subtypes", [])

    # Determine rarity
    rarity = card.get("rarity", "")

    # Determine if holo/reverse holo
    is_holo = any(
        x in rarity.lower()
        for x in ["holo", "rare holo", "amazing rare", "radiant", "shiny"]
    ) if rarity else False

    is_reverse_holo = "reverse" in rarity.lower() if rarity else False

    return {
        "card_id": card_id,
        "name": card.get("name", ""),
        "game": "pokemon",
        "set_id": set_id,
        "collector_number": number,
        "rarity": rarity,

        # Pokemon specific
        "supertype": card.get("supertype"),
        "subtypes": json.dumps(subtypes) if subtypes else None,
        "hp": int(card.get("hp", 0)) if card.get("hp") else None,
        "types": json.dumps(types) if types else None,
        "artist": card.get("artist"),

        # Images
        "image_small": card.get("images", {}).get("small"),
        "image_large": card.get("images", {}).get("large"),

        # Edition flags
        "is_first_edition": is_first_edition,
        "is_unlimited": is_unlimited,
        "is_shadowless": is_shadowless,
        "is_holo": is_holo,
        "is_reverse_holo": is_reverse_holo,
    }


def download_image(url: str, output_path: Path) -> bool:
    """Download an image to the specified path."""
    try:
        req = Request(url, headers={"User-Agent": USER_AGENT})
        with urlopen(req, timeout=30) as response:
            output_path.parent.mkdir(parents=True, exist_ok=True)
            with open(output_path, "wb") as f:
                f.write(response.read())
        return True
    except (HTTPError, URLError, OSError) as e:
        logger.warning(f"Failed to download {url}: {e}")
        return False


def fetch_and_save_to_json(output_path: str, api_key: str | None = None, download_images_dir: str | None = None) -> int:
    """Fetch Pokemon data and save to JSON file."""
    sets = fetch_all_sets(api_key)

    processed_cards = []
    processed_sets = []

    for i, pokemon_set in enumerate(sets):
        set_id = pokemon_set.get("id", "")
        set_name = pokemon_set.get("name", "")
        logger.info(f"Fetching set {i + 1}/{len(sets)}: {set_name} ({set_id})")

        set_data = extract_set_data(pokemon_set)
        processed_sets.append(set_data)

        try:
            cards = fetch_cards_for_set(set_id, api_key)
            logger.info(f"  Found {len(cards)} cards")

            for card in cards:
                try:
                    card_data = extract_card_data(card)
                    processed_cards.append(card_data)

                    # Optionally download images
                    if download_images_dir:
                        image_url = card_data.get("image_large") or card_data.get("image_small")
                        if image_url:
                            image_path = Path(download_images_dir) / set_id / f"{card_data['collector_number']}.png"
                            if not image_path.exists():
                                download_image(image_url, image_path)

                except Exception as e:
                    logger.warning(f"Error processing card {card.get('name', 'unknown')}: {e}")

        except Exception as e:
            logger.error(f"Error fetching cards for set {set_id}: {e}")

    output = {
        "fetched_at": datetime.now(timezone.utc).isoformat(),
        "source": "pokemontcg.io",
        "total_cards": len(processed_cards),
        "total_sets": len(processed_sets),
        "first_edition_sets": list(FIRST_EDITION_SETS),
        "sets": processed_sets,
        "cards": processed_cards,
    }

    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)

    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    logger.info(f"Saved {len(processed_cards)} cards and {len(processed_sets)} sets to {output_path}")
    return len(processed_cards)


def fetch_and_save_to_db(api_key: str | None = None) -> int:
    """Fetch Pokemon data and save to database."""
    from identifier.database import init_db, SessionLocal
    from identifier.database.repository import CardRepository, CardSetRepository

    # Initialize database
    init_db()

    db = SessionLocal()
    try:
        set_repo = CardSetRepository(db)
        card_repo = CardRepository(db)

        sets = fetch_all_sets(api_key)
        cards_processed = 0

        for i, pokemon_set in enumerate(sets):
            set_id = pokemon_set.get("id", "")
            set_name = pokemon_set.get("name", "")
            logger.info(f"Fetching set {i + 1}/{len(sets)}: {set_name} ({set_id})")

            # Upsert set
            set_data = extract_set_data(pokemon_set)
            set_repo.upsert(set_data)

            try:
                cards = fetch_cards_for_set(set_id, api_key)
                logger.info(f"  Found {len(cards)} cards")

                for card in cards:
                    try:
                        card_data = extract_card_data(card)
                        card_repo.upsert(card_data)
                        cards_processed += 1
                    except Exception as e:
                        logger.warning(f"Error processing card {card.get('name', 'unknown')}: {e}")

                # Commit after each set
                db.commit()

            except Exception as e:
                logger.error(f"Error fetching cards for set {set_id}: {e}")

        logger.info(f"Saved {cards_processed} cards and {len(sets)} sets to database")
        return cards_processed

    finally:
        db.close()


def main():
    parser = argparse.ArgumentParser(
        description="Fetch Pokemon card data from PokemonTCG.io API"
    )
    parser.add_argument(
        "--output",
        type=str,
        default="./data/pokemon_cards.json",
        help="Output JSON file path (default: ./data/pokemon_cards.json)",
    )
    parser.add_argument(
        "--to-db",
        action="store_true",
        help="Save directly to database instead of JSON file",
    )
    parser.add_argument(
        "--download-images",
        type=str,
        help="Directory to download card images to",
    )
    parser.add_argument(
        "--api-key",
        type=str,
        default=os.getenv("POKEMONTCG_API_KEY"),
        help="PokemonTCG.io API key for higher rate limits",
    )

    args = parser.parse_args()

    try:
        if args.to_db:
            count = fetch_and_save_to_db(args.api_key)
        else:
            count = fetch_and_save_to_json(args.output, args.api_key, args.download_images)

        logger.info(f"Successfully fetched {count} Pokemon cards from PokemonTCG.io")
        return 0

    except Exception as e:
        logger.error(f"Failed to fetch Pokemon data: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
