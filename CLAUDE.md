# TCG Tracker - Claude Code Guide

## Project Overview

TCG Tracker is a trading card game collection management application supporting Pokemon and Magic: The Gathering cards. It features card scanning via mobile camera with OCR, price tracking with caching, and collection management.

## Architecture

```
tcg-tracker/
├── backend/          # Go REST API server
├── frontend/         # Vue.js 3 web application
└── mobile/           # Flutter mobile app with OCR
```

## Tech Stack

- **Backend**: Go 1.21+, Gin framework, GORM, SQLite
- **Frontend**: Vue.js 3, Vite, Pinia, Vue Router, Tailwind CSS
- **Mobile**: Flutter, Google ML Kit (OCR), camera integration

## Build & Run Commands

### Backend
```bash
cd backend
./run.sh                    # Start server with .env config
# OR manually:
go run cmd/server/main.go   # Start server
go build -o server cmd/server/main.go  # Build binary
go test ./...               # Run tests
```

Environment variables (see `backend/.env.example`):
- `DB_PATH` - SQLite database path
- `PORT` - Server port (default: 8080)
- `FRONTEND_DIST_PATH` - Path to built frontend (enables static serving)
- `POKEMON_PRICE_TRACKER_API_KEY` - API key for Pokemon prices
- `POKEMON_DATA_DIR` - Directory with pokemon-tcg-data
- `POKEMON_PRICE_DAILY_LIMIT` - Daily API request limit (default: 100)

### Frontend
```bash
cd frontend
npm install                 # Install dependencies
npm run dev                 # Development server (port 5173)
npm run build               # Build for production (outputs to dist/)
npm run lint                # Run linter
```

### Mobile
```bash
cd mobile
flutter pub get             # Install dependencies
flutter run                 # Run on connected device/emulator
flutter build apk           # Build Android APK
flutter build ios           # Build iOS app
```

## Key Backend Services

| Service | File | Purpose |
|---------|------|---------|
| `PokemonHybridService` | `internal/services/pokemon_hybrid.go` | Pokemon card search, local data + price API |
| `ScryfallService` | `internal/services/scryfall.go` | MTG card search via Scryfall API |
| `PriceWorker` | `internal/services/price_worker.go` | Background price updates (rate-limited) |
| `OCRParser` | `internal/services/ocr_parser.go` | Parse OCR text to extract card details |

## API Endpoints

Base URL: `http://localhost:8080/api`

### Cards
- `GET /cards/search?q=<query>&game=<pokemon|mtg>` - Search cards
- `GET /cards/:id` - Get card by ID
- `POST /cards/identify` - Identify card from OCR text
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

## Data Flow

1. **Card Search**: User searches → Backend queries local Pokemon data or Scryfall API → Returns cards with cached prices
2. **Card Scanning**: Mobile captures image → ML Kit OCR extracts text → Backend parses OCR text → Matches card by name/set/number
3. **Price Updates**: Background worker runs hourly → Updates ~4 cards per hour → Respects 100 requests/day limit

## Important Implementation Details

### Price Caching
- Pokemon prices are cached in SQLite (24-hour staleness threshold)
- Background worker (`PriceWorker`) updates prices to stay within API limits
- MTG uses Scryfall which has generous limits, no caching needed

### OCR Card Matching
The `OCRParser` extracts from scanned card images:
- Card name (first non-empty line, cleaned)
- Card number (e.g., "025/185" → "25")
- Set code (e.g., "swsh4")
- HP value

Matching priority:
1. Exact match by set code + card number
2. Fuzzy match by name with ranking

### Frontend Serving
The Go backend can serve the Vue.js frontend from `FRONTEND_DIST_PATH`:
- Static assets from `/assets`
- SPA fallback routes non-API paths to `index.html`

## Database

SQLite database with GORM models in `internal/models/`:
- `Card` - Card data with prices, images, metadata
- `CollectionItem` - User's collection entries with quantity, condition

## Pokemon Data

Requires pokemon-tcg-data at `$POKEMON_DATA_DIR/pokemon-tcg-data-master/`:
- Download from: https://github.com/PokemonTCG/pokemon-tcg-data
- Contains card JSON files organized by set
