#!/bin/bash
# Download sample card images for OCR testing

OUTPUT_DIR="${1:-./test_images}"
mkdir -p "$OUTPUT_DIR"

echo "Downloading sample card images to $OUTPUT_DIR..."

# Pokemon cards (high-res)
echo "Downloading Pokemon cards..."
curl -s -o "$OUTPUT_DIR/pokemon_charizard.png" "https://images.pokemontcg.io/smp/SM226_hires.png"
curl -s -o "$OUTPUT_DIR/pokemon_pikachu.png" "https://images.pokemontcg.io/swsh4/25_hires.png"
curl -s -o "$OUTPUT_DIR/pokemon_mewtwo.png" "https://images.pokemontcg.io/sm35/67_hires.png"
curl -s -o "$OUTPUT_DIR/pokemon_blastoise.png" "https://images.pokemontcg.io/det1/16_hires.png"

# MTG cards (from Scryfall)
echo "Downloading MTG cards..."
curl -s -o "$OUTPUT_DIR/mtg_lightning_bolt.jpg" "https://cards.scryfall.io/large/front/f/2/f29ba16f-c8fb-42fe-aabf-87b71b5e5a02.jpg"
curl -s -o "$OUTPUT_DIR/mtg_counterspell.jpg" "https://cards.scryfall.io/large/front/1/b/1b73577a-8ca1-41d7-9b2b-be70b42f23e0.jpg"
curl -s -o "$OUTPUT_DIR/mtg_dark_ritual.jpg" "https://cards.scryfall.io/large/front/9/5/95f27eeb-6f14-4db3-adb9-9be5ed76b34b.jpg"

echo "Downloaded files:"
ls -la "$OUTPUT_DIR"

echo ""
echo "To run OCR on these images:"
echo "  tesseract $OUTPUT_DIR/pokemon_pikachu.png stdout"
echo ""
echo "To test identification with extracted text:"
echo "  curl -X POST http://localhost:8080/api/cards/identify \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"text\": \"Pikachu\", \"game\": \"pokemon\"}'"
