#!/bin/bash
# OCR Test Script for TCG Tracker
# Tests the card identification endpoint with various inputs

API_URL="${API_URL:-http://localhost:8080}"

echo "=== TCG Tracker OCR Identification Tests ==="
echo "API URL: $API_URL"
echo ""

# Function to test identification
test_identify() {
    local name="$1"
    local text="$2"
    local game="$3"

    echo "Test: $name"
    result=$(curl -s -X POST "$API_URL/api/cards/identify" \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"$text\", \"game\": \"$game\"}")

    count=$(echo "$result" | jq -r '.total_count // 0')
    first=$(echo "$result" | jq -r '.cards[0].name // "No match"')

    if [ "$count" -gt 0 ]; then
        echo "  ✓ Found $count matches. Best: $first"
    else
        echo "  ✗ No matches found"
    fi
    echo ""
}

echo "--- Pokemon Card Tests ---"
test_identify "Clean card name" "Charizard" "pokemon"
test_identify "Card with variant" "Pikachu VMAX" "pokemon"
test_identify "EX card" "Mewtwo EX" "pokemon"
test_identify "Classic card" "Blastoise" "pokemon"
test_identify "Partial name" "dragon" "pokemon"

echo "--- MTG Card Tests ---"
test_identify "Iconic card" "Black Lotus" "mtg"
test_identify "Common card" "Lightning Bolt" "mtg"
test_identify "Artifact" "Sol Ring" "mtg"
test_identify "Creature type" "Goblin" "mtg"

echo "--- Simulated OCR Errors ---"
test_identify "Missing letters" "Pikach" "pokemon"
test_identify "Extra spaces" "Light ning  Bolt" "mtg"
test_identify "Number substitution" "P1kachu" "pokemon"

echo "=== Tests Complete ==="
