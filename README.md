# TCG Tracker

A trading card collection tracker for Magic: The Gathering and Pokemon cards with camera-based card scanning, Gemini AI-powered identification, and price tracking.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│  Flutter App    │     │   Vue.js Web    │
│  (Mobile Scan)  │     │   (Dashboard)   │
└────────┬────────┘     └────────┬────────┘
         │                       │
         └───────────┬───────────┘
                     │ REST API
              ┌──────▼──────┐
              │   Go API    │
              │   Server    │
              └──────┬──────┘
                     │
    ┌────────────────┼────────────────┬────────────────┐
    │                │                │                │
┌───▼───┐      ┌─────▼─────┐   ┌──────▼──────┐   ┌─────▼─────┐
│SQLite │      │ Scryfall  │   │ Gemini API  │   │ JustTCG   │
│  DB   │      │   API     │   │  (Vision)   │   │   API     │
└───────┘      └───────────┘   └─────────────┘   └───────────┘
                                                        │
                                                 ┌──────▼──────┐
                                                 │ Prometheus  │───► Grafana
                                                 │  /metrics   │
                                                 └─────────────┘
```

## Features

- **Gemini AI Card Identification**: Upload card images for automatic identification using Gemini Vision with 12+ specialized tools (search, lookup, image comparison, set info)
- **Bulk Import**: Upload up to 200 card images at once via the web UI, with background Gemini processing (10 concurrent), categorized error messages with suggestions, review/edit results, then batch-add to collection
- **Multi-Language Support**: Automatically detects card language (Japanese, German, French, Italian) with language-specific pricing
- **Card Search**: Search for MTG and Pokemon cards using external APIs
- **MTG 2-Phase Selection**: When scanning MTG cards, browse all printings grouped by set and select the exact variant (foil, showcase, borderless, etc.)
- **Collection Management**: Add, update, and remove cards from your collection
- **Price Tracking**: View current market prices with automatic refresh and batch updates
- **TCGPlayerID Sync**: Admin tools to prepopulate Pokemon TCGPlayerIDs for faster pricing
- **Mobile Scanning**: Use your phone camera to scan and identify cards
- **Prometheus Metrics**: Export metrics for monitoring with Grafana dashboards

## Prerequisites

- **Go** 1.24+ (for backend)
- **Node.js** 20+ (for frontend)
- **Flutter** 3.38+ (for mobile app)
- **Docker** (recommended for deployment)
- **Gemini API Key** (required for card scanning)

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Production: Pull pre-built images from GHCR
GOOGLE_API_KEY=your-gemini-key docker compose up -d

# Local development: Build images locally
docker compose -f docker-compose.yml -f docker-compose.local.yml up --build
```

Services will be available at:
- App (Frontend + API): http://localhost:3080
- Prometheus Metrics: http://localhost:3080/metrics

### Docker Images

Images are automatically built and pushed to GitHub Container Registry on each commit to main:

| Image | Description |
|-------|-------------|
| `ghcr.io/seavey-org/tcg-tracker/app` | Combined Go API + Vue.js frontend |

The `app` image is a multi-stage build that compiles the Vue.js frontend and Go backend into a single image, with the Go server serving the static frontend files.

**Tags:**
- `latest` - Most recent main branch build
- `<commit-sha>` - Specific commit (for rollback)

**Rollback to previous version:**
```bash
IMAGE_TAG=abc123def docker compose up -d
```

### Option 2: Manual Setup

#### 1. Backend (Go API)

```bash
cd backend
go mod tidy
GOOGLE_API_KEY=your-gemini-key go run cmd/server/main.go
```

The API will start on `http://localhost:8080`.

Environment variables:
- `PORT` - Server port (default: 8080)
- `DB_PATH` - SQLite database path (default: ./tcg_tracker.db)
- `POKEMON_DATA_DIR` - Pokemon TCG data directory
- `GOOGLE_API_KEY` - Gemini API key for card identification (**required** for scanning)
- `ADMIN_KEY` - Admin key for collection modification (optional, auth disabled if not set)
- `JUSTTCG_API_KEY` - JustTCG API key for condition-based pricing
- `JUSTTCG_DAILY_LIMIT` - Daily API request limit (default: 100)
- `JUSTTCG_MONTHLY_LIMIT` - Monthly API request limit (default: 1000)
- `SYNC_TCGPLAYER_IDS_ON_STARTUP` - Set to "true" to sync missing Pokemon TCGPlayerIDs on startup
- `BULK_IMPORT_CONCURRENCY` - Number of concurrent Gemini calls for bulk import (default: 10)
- `BULK_IMPORT_IMAGES_DIR` - Directory for bulk import images (default: ./data/bulk_import_images)

#### 2. Frontend (Vue.js Web App)

```bash
cd frontend
npm install
npm run dev
```

The web app will start on `http://localhost:5173`.

#### 3. Mobile App (Flutter)

```bash
cd mobile
flutter pub get
flutter run
```

Configure the server URL in settings to point to your backend IP.

## API Endpoints

### Cards
- `GET /api/cards/search?q={query}&game={mtg|pokemon}` - Search for cards
- `GET /api/cards/search/grouped?q={query}&game={mtg|pokemon}&sort={release_date|release_date_asc|name|cards}` - Search cards grouped by set
- `GET /api/cards/:id?game={mtg|pokemon}` - Get card details
- `GET /api/cards/:id/prices` - Get condition-specific prices for a card
- `POST /api/cards/identify` - Identify card from OCR text
- `POST /api/cards/identify-image` - Identify card from uploaded image
- `GET /api/cards/ocr-status` - Check if server-side OCR is available

### Auth
- `GET /api/auth/status` - Check if authentication is enabled
- `POST /api/auth/verify` - Verify admin key

### Collection
- `GET /api/collection` - Get all collection items (flat list)
- `GET /api/collection/grouped` - Get collection grouped by card with variants
- `POST /api/collection` - Add card to collection (🔒)
- `PUT /api/collection/:id` - Update collection item with smart split/merge/reassign (🔒)
- `DELETE /api/collection/:id` - Remove from collection (🔒)
- `GET /api/collection/stats` - Get collection statistics
- `GET /api/collection/stats/history` - Get historical collection value snapshots (for charting)
- `POST /api/collection/refresh-prices` - Trigger immediate price update batch (up to 100 cards) (🔒)

### Prices
- `GET /api/prices/status` - Get pricing quota status and next update time

### Admin (🔒)
- `POST /api/admin/sync-tcgplayer-ids` - Start async TCGPlayerID sync for collection cards
- `POST /api/admin/sync-tcgplayer-ids/blocking` - Sync TCGPlayerIDs and wait for completion
- `POST /api/admin/sync-tcgplayer-ids/set/:setName` - Sync TCGPlayerIDs for a specific set
- `GET /api/admin/sync-tcgplayer-ids/status` - Check sync status and quota

### Bulk Import (🔒)
- `POST /api/bulk-import/jobs` - Upload images and create bulk import job (multipart, max 200)
- `GET /api/bulk-import/jobs` - Get current/most recent job
- `GET /api/bulk-import/jobs/:id` - Get job with all items
- `PUT /api/bulk-import/jobs/:id/items/:itemId` - Update item (select card, change condition)
- `POST /api/bulk-import/jobs/:id/confirm` - Add confirmed items to collection
- `DELETE /api/bulk-import/jobs/:id` - Cancel and delete job
- `GET /api/bulk-import/search` - Search cards for manual selection

*🔒 = Requires admin key if `ADMIN_KEY` is set*

### Monitoring
- `GET /health` - Service health check
- `GET /metrics` - Prometheus metrics endpoint

### Identifier Service (port 8099)
- `GET /health` - Service health and GPU status
- `POST /ocr` - OCR text extraction with auto-rotation

## Project Structure

```
tcg-tracker/
├── backend/                 # Go API server
│   ├── cmd/server/          # Main entry point
│   └── internal/
│       ├── api/             # HTTP handlers and routes
│       ├── database/        # SQLite setup
│       ├── metrics/         # Prometheus metrics
│       ├── models/          # Data models
│       └── services/        # External API services
├── frontend/                # Vue.js web application
│   └── src/
│       ├── components/      # Reusable Vue components
│       ├── views/           # Page components
│       ├── services/        # API client
│       └── stores/          # Pinia state management
├── identifier/              # Python OCR service
│   ├── app.py               # FastAPI application with /health and /ocr endpoints
│   └── ocr_engine.py        # EasyOCR singleton with GPU acceleration
├── mobile/                  # Flutter mobile app
│   └── lib/
│       ├── models/          # Data models
│       ├── screens/         # App screens
│       └── services/        # API and OCR services
├── monitoring/              # Monitoring configuration
│   └── grafana-dashboard.json # Pre-built Grafana dashboard
└── deployment/              # Deployment configs
    ├── tcg-tracker.service  # Backend systemd service
    └── tcg-identifier.service # Identifier systemd service
```

## OCR Pipeline

The system uses a two-tier OCR approach for card identification:

1. **Server-side OCR** (preferred): GPU-accelerated EasyOCR extracts text from card images
2. **Client-side OCR** (fallback): Google ML Kit on mobile device if server unavailable
3. **Card Matching**: Inverted index enables fast matching (~1.5ms for good OCR)
4. **Reliability Fallback**: Falls back to full card scan if index match score is low

**Japanese Card Support:**
- Server-side OCR with `OCR_LANGUAGES=ja,en` is required for accurate Japanese card scanning
- Client-side OCR is configured for Latin script only
- **Gemini Vision (preferred)**: For Japanese cards, the system uses Gemini 3 Flash vision to identify cards directly from images, bypassing OCR translation issues entirely
- Japanese cards with English names (e.g., "ピカチュウ Pikachu") can match by English text
- Japanese-only cards rely on set code + card number matching
- **User-Confirmed Caching**: When adding a Japanese card to collection, the OCR text → card ID mapping is cached for instant lookups on future scans (normalized to handle OCR variations)
- **Hybrid Translation (fallback)**: For low-confidence matches, Japanese text is translated using:
  1. Static map (1025 Pokemon + common trainer cards)
  2. SQLite cache (avoids repeat API calls)
  3. Gemini 3 Flash text (if GOOGLE_API_KEY configured)
  4. Google Cloud Translation API (fallback if Gemini unavailable or low confidence)

## Japanese Pokemon Card Tools

The backend includes tools for managing Japanese Pokemon card data:

### Fetching Japanese Card Data

Japanese Pokemon cards are a separate product line on TCGPlayer with unique TCGPlayerIDs. Use the fetch tool to populate Japanese card data from JustTCG:

```bash
cd backend

# Fetch a specific set
go run cmd/fetch-japanese-cards/main.go -output=./data/pokemon-tcg-data/pokemon-tcg-data-japan -set=gold-silver-to-a-new-world-pokemon-japan

# Resume fetching all sets (skips already-fetched)
go run cmd/fetch-japanese-cards/main.go -output=./data/pokemon-tcg-data/pokemon-tcg-data-japan -resume
```

**Note:** JustTCG has 420 Japanese sets. Due to API rate limits (50 req/min, 1000 req/day), fetching all sets takes ~2 days.

### Migrating Collection Items

If you have Japanese cards in your collection that were added with English card IDs, migrate them to proper Japanese IDs for accurate pricing:

```bash
cd backend

# Preview migration
go run cmd/migrate-japanese-collection/main.go -db=./data/tcg_tracker.db -data=./data -dry-run

# Execute migration with interactive prompts
go run cmd/migrate-japanese-collection/main.go -db=./data/tcg_tracker.db -data=./data -execute
```

### Language-Specific Pricing Limitations

| Language | Pokemon | MTG |
|----------|---------|-----|
| Japanese | Separate TCGPlayerIDs, requires `jp-*` card IDs | Works via ScryfallID |
| German/French/Italian | **No pricing available** (falls back to English) | Works via ScryfallID |

## External APIs

### Scryfall (MTG)
- No API key required
- Rate limit: 10 requests/second
- Documentation: https://scryfall.com/docs/api

### JustTCG (Pricing)
- Provides condition-specific pricing for Pokemon and MTG cards
- API key required
- Free tier: 100 requests/day, 1000 requests/month, 20 cards/request
- Both daily and monthly limits enforced; configure via `JUSTTCG_DAILY_LIMIT` and `JUSTTCG_MONTHLY_LIMIT`
- Batch pricing uses TCGPlayerIDs (Pokemon) or ScryfallIDs (MTG) for up to 20 cards per request
- Pokemon Japan is a separate game with unique TCGPlayerIDs

## Monitoring

The backend exposes Prometheus metrics at `/metrics` for monitoring:

### Available Metrics
- `tcg_collection_cards_total` - Total cards in collection
- `tcg_collection_value_usd` - Total collection value
- `tcg_price_updates_total` - Price updates counter
- `tcg_price_queue_size` - Cards waiting for price refresh
- `tcg_justtcg_quota_remaining` - API quota remaining
- `tcg_http_requests_total` - HTTP request counter by path/method/status
- `tcg_http_request_duration_seconds` - Request latency histogram
- `tcg_translation_requests_total` - Translation requests by source (static/cache/api)
- `tcg_translation_cache_hits_total` - Translation cache hit count
- `tcg_translation_api_latency_seconds` - Google Cloud Translation API latency
- `tcg_gemini_requests_total` - Gemini API requests
- `tcg_gemini_api_latency_seconds` - Gemini API latency
- `tcg_gemini_confidence` - Gemini response confidence scores
- `tcg_translation_decisions_total` - Translation source decisions (static/cache/gemini/google_api)

### Grafana Dashboard
Import the pre-built dashboard from `monitoring/grafana-dashboard.json` to visualize:
- Collection overview (card count, total value, breakdown by game)
- Price update status (queue size, update rate, batch duration)
- JustTCG API quota usage
- HTTP traffic and latency
