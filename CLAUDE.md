# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TCG Tracker is a trading card game collection management application supporting Pokemon and Magic: The Gathering cards. It features card scanning via mobile camera with OCR, price tracking with caching, and collection management.

## Architecture

```
tcg-tracker/
├── backend/          # Go REST API server
├── frontend/         # Vue.js 3 web application
├── mobile/           # Flutter mobile app with OCR (see mobile/CLAUDE.md for details)
└── identifier/       # Python service for server-side OCR (EasyOCR)
```

## Tech Stack

- **Backend**: Go 1.24+, Gin framework, GORM, SQLite
- **Frontend**: Vue.js 3, Vite, Pinia, Vue Router, Tailwind CSS
- **Mobile**: Flutter, Google ML Kit (OCR fallback), camera integration
- **Identifier**: Python 3.11+, FastAPI, EasyOCR (GPU-accelerated), OpenCV

## Build & Run Commands

### Docker Compose (Recommended)
```bash
# Production: Pull pre-built images from GHCR
docker compose up -d

# Local development: Build images locally
docker compose -f docker-compose.yml -f docker-compose.local.yml up --build

# Production with GPU support for identifier
docker compose -f docker-compose.yml -f docker-compose.gpu.yml up -d

# Deploy specific version (for rollback)
IMAGE_TAG=abc123def docker compose up -d
```

### CI/CD & Docker Images
- Images are built and pushed to GHCR on each commit to main
- Images: `ghcr.io/codyseavey/tcg-tracker/{backend,frontend,identifier}`
- Tags: `latest` (most recent) and `<commit-sha>` (for rollback)
- Deploy pulls images instead of building on server

### Backend
```bash
cd backend
go run cmd/server/main.go             # Start server
go build -o server cmd/server/main.go # Build binary
go test ./...                         # Run all tests
go test -v ./internal/services/...    # Run specific package tests
go test -v -run TestParseOCRText ./internal/services/...  # Run single test by name
golangci-lint run                     # Run linter
```

Environment variables (see `backend/.env.example`):
- `DB_PATH` - SQLite database path
- `PORT` - Server port (default: 8080)
- `FRONTEND_DIST_PATH` - Path to built frontend (enables static serving)
- `POKEMON_DATA_DIR` - Directory for pokemon-tcg-data (auto-downloaded on first run)
- `JUSTTCG_API_KEY` - JustTCG API key for condition-based pricing (optional)
- `JUSTTCG_DAILY_LIMIT` - Daily API request limit (default: 100)

### Frontend
```bash
cd frontend
npm install                 # Install dependencies
npm run dev                 # Development server (port 5173)
npm run build               # Build for production (outputs to dist/)
npm run lint                # Run ESLint
npm run lint:fix            # Run ESLint with auto-fix
```

### Mobile
```bash
cd mobile
flutter pub get             # Install dependencies
flutter run                 # Run on connected device/emulator
flutter test                # Run all tests
flutter analyze             # Run linter
flutter build apk           # Build Android APK
```

### Identifier Service (Optional)
```bash
cd identifier
python -m venv venv && source venv/bin/activate
pip install -r requirements.txt

uvicorn identifier.app:app --host 127.0.0.1 --port 8099  # Start server
pytest tests/                                             # Run tests
```

Environment variables:
- `HOST` - Bind address (default: 127.0.0.1)
- `PORT` - Server port (default: 8099)
- `USE_GPU` - Enable GPU acceleration (default: 1)

## Key Backend Services

| Service | File | Purpose |
|---------|------|---------|
| `PokemonHybridService` | `internal/services/pokemon_hybrid.go` | Pokemon card search with local data |
| `ScryfallService` | `internal/services/scryfall.go` | MTG card search via Scryfall API |
| `JustTCGService` | `internal/services/justtcg.go` | Condition-based pricing from JustTCG API |
| `PriceService` | `internal/services/price_service.go` | Unified price fetching with fallback chain |
| `TCGdexService` | `internal/services/tcgdex.go` | Pokemon card pricing fallback (TCGdex API) |
| `PriceWorker` | `internal/services/price_worker.go` | Background price updates (20 cards/batch hourly) |
| `OCRParser` | `internal/services/ocr_parser.go` | Parse OCR text to extract card details |
| `ServerOCRService` | `internal/services/server_ocr.go` | Server-side OCR using identifier service (EasyOCR) |

### Identifier Service (Python)

The identifier service provides server-side OCR text extraction using EasyOCR.

| Module | File | Purpose |
|--------|------|---------|
| `app` | `identifier/app.py` | FastAPI app with /health and /ocr endpoints |
| `OCREngine` | `identifier/ocr_engine.py` | EasyOCR singleton with GPU acceleration |

Features:
- **GPU acceleration**: Automatically uses CUDA if available (10-50x faster than CPU)
- **Singleton pattern**: OCR engine initialized once at startup for fast inference
- **Auto-rotation**: Uses EasyOCR's `rotation_info` for efficient orientation detection
- **Image downscaling**: Images downscaled to 1280px max dimension for performance

## API Endpoints

Base URL: `http://localhost:8080/api`

### Cards
- `GET /cards/search?q=<query>&game=<pokemon|mtg>` - Search cards
- `GET /cards/:id` - Get card by ID
- `GET /cards/:id/prices` - Get condition-specific prices for a card
- `POST /cards/identify` - Identify card from OCR text (client-side OCR)
- `POST /cards/identify-image` - Identify card from uploaded image (server-side OCR)
- `GET /cards/ocr-status` - Check if server-side OCR is available
- `POST /cards/:id/refresh-price` - Refresh single card price

### Collection
- `GET /collection` - Get user's collection
- `POST /collection` - Add card to collection
- `PUT /collection/:id` - Update collection item
- `DELETE /collection/:id` - Remove from collection
- `GET /collection/stats` - Get collection statistics
- `POST /collection/refresh-prices` - Queue background price refresh

### Prices
- `GET /prices/status` - Get API quota status

### Health
- `GET /health` - Health check

### Identifier Service (port 8099)
- `GET /health` - Service health and GPU status
- `POST /ocr` - OCR text extraction with auto-rotation

## Data Flow

1. **Card Search**: User searches → Backend queries local Pokemon data or Scryfall API → Returns cards with cached prices
2. **Card Scanning (Server OCR)**: Mobile captures image → Backend sends to identifier service → EasyOCR extracts text → Backend parses OCR text → Matches card by name/set/number
3. **Card Scanning (Fallback)**: If server OCR unavailable → Mobile uses Google ML Kit locally → Sends text to backend for parsing
4. **Price Updates**: Background worker runs hourly → Updates 20 cards per batch via JustTCG (with TCGdex/Scryfall fallback)

## Important Implementation Details

### Price Caching
- Condition-specific prices (NM, LP, MP, HP, DMG) are stored in `card_prices` table
- Base prices (NM only) kept in `cards` table for backward compatibility
- `PriceService` provides unified price fetching with fallback chain:
  1. Check database cache (fresh within 24 hours)
  2. Try JustTCG API (condition-specific pricing for Pokemon and MTG)
  3. Fallback to TCGdex (Pokemon) or Scryfall (MTG) for NM prices
  4. Return stale cached price if all else fails
- Background worker updates 20 cards per batch hourly (both Pokemon and MTG)
- Collection stats use condition-appropriate prices (e.g., LP card uses LP price)

### OCR Card Matching
The `OCRParser` extracts from scanned card images:
- Card name (first non-empty line, cleaned)
- Card number (e.g., "025/185" → "25", "TG17/TG30" → "TG17")
- Set code (e.g., "swsh4") - via direct detection, set name mapping, or total inference
- HP value (Pokemon)
- Foil indicators (V, VMAX, VSTAR, GX, EX, holo, full art, etc.)
- Rarity (Illustration Rare, Secret Rare, etc.)

Matching priority:
1. Exact match by set code + card number
2. Fuzzy match by name with ranking

### Inverted Index for Card Matching
The `PokemonHybridService` uses an inverted index for fast full-text card matching:

**Index Structure:**
- `wordIndex map[string][]int` - maps words to card indices
- Built at startup from ~20,000 cards
- ~13,000 unique indexed words
- ~20MB additional memory

**Performance:**
| Scenario | Time | Notes |
|----------|------|-------|
| Good OCR (index hit) | ~1.5ms | 18x faster than full scan |
| With set filter | ~37μs | 720x faster than full scan |
| Poor OCR (fallback) | ~27ms | Falls back to full scan |

**Reliability Fallback:**
- If best match score < 500, falls back to full scan of all cards
- Ensures no false negatives from OCR errors
- Poor OCR still works, just slower

**Key Functions:**
- `findCandidatesByIndex()` - finds cards matching any OCR word via index
- `scoreCards()` - scores specific card indices against OCR text
- `scoreAllCards()` - full scan fallback for reliability

### OCR Processing Options
Two OCR processing paths are available:
1. **Server-side OCR** (preferred): Mobile uploads image to `/cards/identify-image` → Go backend calls identifier service `/ocr` endpoint → EasyOCR extracts text → Backend parses and matches card
   - Uses GPU-accelerated EasyOCR for better accuracy and speed
   - Check availability via `/cards/ocr-status`
2. **Client-side OCR** (fallback): If server OCR unavailable → Mobile uses Google ML Kit locally → Sends extracted text to `/cards/identify`

### Frontend Serving
The Go backend can serve the Vue.js frontend from `FRONTEND_DIST_PATH`:
- Static assets from `/assets`
- SPA fallback routes non-API paths to `index.html`

## Database

SQLite database with GORM models in `internal/models/`:
- `Card` - Card data with prices, images, metadata
- `CardPrice` - Condition-specific prices (NM, LP, MP, HP, DMG) for each card/foil combo
- `CollectionItem` - User's collection entries with quantity, condition

## Pokemon Data

Pokemon TCG data is stored at `$POKEMON_DATA_DIR/pokemon-tcg-data-master/`:
- Auto-downloaded from GitHub on first server startup
- Contains card JSON files organized by set (e.g., `cards/en/swsh4.json`)
