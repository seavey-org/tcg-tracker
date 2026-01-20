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
- Images: `ghcr.io/codyseavey/tcg-tracker/{app,identifier}`
- Tags: `latest` (most recent) and `<commit-sha>` (for rollback)
- Deploy pulls images instead of building on server
- The `app` image is a combined frontend+backend build (Go serves the Vue.js static files)

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
- `JUSTTCG_DAILY_LIMIT` - Daily API request limit (default: 1000)
- `ADMIN_KEY` - Admin key for collection modification (optional, disables auth if not set)
- `SYNC_TCGPLAYER_IDS_ON_STARTUP` - Set to "true" to auto-sync TCGPlayerIDs on startup

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
| `JustTCGService` | `internal/services/justtcg.go` | Condition-based pricing from JustTCG API (sole price source) |
| `PriceService` | `internal/services/price_service.go` | Unified price fetching from JustTCG |
| `PriceWorker` | `internal/services/price_worker.go` | Background price updates with priority queue (user requests, then collection) |
| `OCRParser` | `internal/services/ocr_parser.go` | Parse OCR text to extract card details |
| `ServerOCRService` | `internal/services/server_ocr.go` | Server-side OCR using identifier service (EasyOCR) |
| `AdminKeyAuth` | `internal/middleware/auth.go` | Admin key authentication middleware |
| `SnapshotService` | `internal/services/snapshot_service.go` | Daily collection value snapshots for historical tracking |
| `ImageStorageService` | `internal/services/image_storage.go` | Store and retrieve scanned card images |
| `TCGPlayerSyncService` | `internal/services/tcgplayer_sync.go` | Bulk sync TCGPlayerIDs from JustTCG for Pokemon cards |

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

### Auth
- `GET /auth/status` - Check if authentication is enabled
- `POST /auth/verify` - Verify admin key (returns valid: true/false)

### Collection
- `GET /collection` - Get user's collection (flat list)
- `GET /collection/grouped?q=<search>&game=<pokemon|mtg>&sort=<added_at|name|price_updated>` - Get collection grouped by card_id with variants summary (supports search and sorting)
- `POST /collection` - Add card to collection (🔒 requires admin key)
- `PUT /collection/:id` - Update collection item with smart split/merge (🔒 requires admin key)
- `DELETE /collection/:id` - Remove from collection (🔒 requires admin key)
- `GET /collection/stats` - Get collection statistics
- `GET /collection/stats/history` - Get historical collection value snapshots (for charting)
- `POST /collection/refresh-prices` - Immediately refresh prices for collection cards (up to 100 per batch) (🔒 requires admin key)

### Prices
- `GET /prices/status` - Get API quota status and next update time

### Admin (🔒 requires admin key)
- `POST /admin/sync-tcgplayer-ids` - Start async TCGPlayerID sync for collection cards
- `POST /admin/sync-tcgplayer-ids/blocking` - Sync TCGPlayerIDs and wait for completion
- `POST /admin/sync-tcgplayer-ids/set/:setName` - Sync TCGPlayerIDs for a specific set
- `GET /admin/sync-tcgplayer-ids/status` - Check sync status and quota

### Health & Monitoring
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics endpoint (for Grafana)

### Identifier Service (port 8099)
- `GET /health` - Service health and GPU status
- `POST /ocr` - OCR text extraction with auto-rotation

## Data Flow

1. **Card Search**: User searches → Backend queries local Pokemon data or Scryfall API → Returns cards with cached prices
2. **Card Scanning (Server OCR)**: Mobile captures image → Backend sends to identifier service → EasyOCR extracts text → Backend parses OCR text → Matches card by name/set/number
3. **Card Scanning (Fallback)**: If server OCR unavailable → Mobile uses Google ML Kit locally → Sends text to backend for parsing
4. **Price Updates**: Background worker runs every 15 minutes → Updates up to 100 cards per batch via JustTCG (skips when quota exhausted)

## Important Implementation Details

### Authentication
Optional admin key authentication protects collection modification endpoints:
- Set `ADMIN_KEY` environment variable to enable (generate with `openssl rand -base64 32`)
- If `ADMIN_KEY` is not set, all operations are allowed (backwards compatible for local dev)
- Protected endpoints: `POST /collection`, `PUT /collection/:id`, `DELETE /collection/:id`, `POST /collection/refresh-prices`
- Key sent via `Authorization: Bearer <key>` header
- Constant-time comparison prevents timing attacks
- Frontend/mobile prompt for key on 401 response, store in localStorage/SecureStorage

### Price Caching
- Condition and printing-specific prices (NM, LP, MP, HP, DMG) stored in `card_prices` table
- Prices keyed by card_id + condition + printing (Normal, Foil, 1st Edition, etc.)
- Base prices (NM only) kept in `cards` table for backward compatibility
- All prices come from JustTCG API (no fallback to other sources for uniform data)
- `PriceService` returns cached data only (no live API calls)
- `Card.GetPrice()` fallback chain (handles holo-only cards where JustTCG stores price as "Normal"):
  1. Exact condition+printing match from `card_prices`
  2. NM price for same printing (if condition is not NM)
  3. For foil variants (1st Ed, Reverse Holo): try standard Foil price
  4. Cross-printing fallback: Foil→Normal or Normal→Foil (for holo-only cards)
  5. Base prices (`PriceFoilUSD` for foil variants, `PriceUSD` otherwise)
  6. Final cross-fallback if primary base price is zero
- Viewing card prices (`GET /cards/:id/prices`) auto-queues refresh if stale

### Price Worker
The background price worker (`PriceWorker`) updates prices with priority ordering:
1. **User-requested refreshes** - Cards queued via `/cards/:id/refresh-price` or stale price views
2. **Collection cards without prices** - New additions needing initial pricing
3. **Collection cards with oldest prices** - Stale cache refresh

**JustTCG API Batching:**
- **MTG cards**: Use efficient batch POST endpoint with `scryfallId` (1 request for up to 100 cards)
- **Pokemon cards**: Set sync discovers `tcgplayerId`, then batch POST for prices
- **Cached TCGPlayerIDs**: Once discovered, Pokemon cards use batch POST for subsequent lookups
- MTG cards can always batch because our `Card.ID` IS the Scryfall UUID
- Pokemon cards cache `TCGPlayerID` in the Card model after set sync

**JustTCG API Limits (Paid Tier):**
- 1000 requests/day, 50 requests/minute, 100 cards/request
- Set sync: ~2-3 requests per set (100 cards/page)
- Price batch: 1 request for up to 100 cards

**TCGPlayerID Bulk Sync:**
- `SYNC_TCGPLAYER_IDS_ON_STARTUP=true` triggers sync 5 seconds after server start
- Fetches TCGPlayerIDs by set from JustTCG (~100 cards per request)
- **Caveat**: Startup sync requires available quota; if exhausted, it stops and doesn't retry
- Use `/api/admin/sync-tcgplayer-ids` to manually trigger sync when quota is available

**Price Worker Set-First Strategy:**
- Before each batch, worker checks for Pokemon cards missing TCGPlayerIDs
- Groups cards by set, syncs each set first (~2-3 requests per set)
- Then performs batch POST for prices (1 request for up to 100 cards)
- **No individual GET requests** - cards without TCGPlayerIDs after set sync are skipped
- Example: 100 Pokemon cards from 3 sets = ~9 set syncs + 1 batch = **~10 requests** (not 100)

**Immediate Refresh:**
- `POST /collection/refresh-prices` triggers an immediate batch update (doesn't wait for scheduled interval)
- Useful for users who want prices updated right away after adding cards
- Still respects rate limits and daily quota

Worker skips updates when daily quota is exhausted (resets at midnight).
Collection stats use condition and printing-appropriate prices.

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

### Collection Grouping and Smart Updates
The collection supports two viewing modes:
1. **Flat list** (`GET /collection`) - Returns individual collection items
2. **Grouped view** (`GET /collection/grouped`) - Groups items by card_id with:
   - Total quantity and value across all variants
   - Breakdown by printing+condition variant
   - Scanned card count
   - Individual items for editing

**Smart Split/Merge on Update (`PUT /collection/:id`):**
When changing condition or printing on a collection item:
- **Scanned items** (with `scanned_image_path`): Always stay individual, updated in place
- **Stack with qty > 1**: Splits 1 card off with new attributes, merges into existing stack if one exists
- **Single item (qty=1)**: Merges into existing stack with matching attributes, or updates in place

Response includes `operation` field: `"updated"`, `"split"`, or `"merged"`

**Add to Collection Logic:**
- Cards with scanned images always create individual entries (qty=1)
- Non-scanned cards merge into existing stacks with matching card_id+condition+printing

### Frontend Serving
The Go backend can serve the Vue.js frontend from `FRONTEND_DIST_PATH`:
- Static assets from `/assets`
- SPA fallback routes non-API paths to `index.html`

## Database

SQLite database with GORM models in `internal/models/`:
- `Card` - Card data with prices, images, metadata
- `CardPrice` - Condition and printing-specific prices (NM, LP, MP, HP, DMG) for each card/printing combo
- `CollectionItem` - User's collection entries with quantity, condition, printing type

### Printing Types
Cards support multiple printing variants via the `PrintingType` enum:
- `Normal` - Standard non-foil printing
- `Foil` - Holographic/foil finish
- `1st Edition` - First edition cards (Pokemon)
- `Reverse Holofoil` - Reverse holo pattern
- `Unlimited` - Unlimited edition (Pokemon)

## Pokemon Data

Pokemon TCG data is stored at `$POKEMON_DATA_DIR/pokemon-tcg-data-master/`:
- Auto-downloaded from GitHub on first server startup
- Contains card JSON files organized by set (e.g., `cards/en/swsh4.json`)
