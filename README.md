# TCG Tracker

A trading card collection tracker for Magic: The Gathering and Pokemon cards with camera-based card scanning, ML-powered identification, and price tracking.

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
              │   Go API    │────────┐
              │   Server    │        │
              └──────┬──────┘        │
                     │               │
    ┌────────────────┼───────────────┼────────────────┐
    │                │               │                │
┌───▼───┐      ┌─────▼─────┐   ┌─────▼─────┐   ┌──────▼──────┐
│SQLite │      │ Scryfall  │   │ JustTCG   │   │ Identifier  │
│  DB   │      │   API     │   │   API     │   │   Service   │
└───────┘      └───────────┘   └───────────┘   └─────────────┘
                                      │
                               ┌──────▼──────┐
                               │ Prometheus  │───► Grafana
                               │  /metrics   │
                               └─────────────┘
```

## Features

- **Card Search**: Search for MTG and Pokemon cards using external APIs
- **OCR Card Identification**: Upload card images for automatic identification using GPU-accelerated OCR
- **MTG 2-Phase Selection**: When scanning MTG cards, browse all printings grouped by set and select the exact variant (foil, showcase, borderless, etc.)
- **Collection Management**: Add, update, and remove cards from your collection
- **Price Tracking**: View current market prices with automatic refresh and batch updates
- **TCGPlayerID Sync**: Admin tools to prepopulate Pokemon TCGPlayerIDs for faster pricing
- **Mobile Scanning**: Use your phone camera to scan and identify cards
- **Fast Card Matching**: Inverted index enables sub-2ms card matching for good OCR
- **Prometheus Metrics**: Export metrics for monitoring with Grafana dashboards

## Prerequisites

- **Go** 1.24+ (for backend)
- **Node.js** 20+ (for frontend)
- **Flutter** 3.38+ (for mobile app)
- **Python** 3.11+ (for identifier service)
- **Docker** (recommended for deployment)

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Production: Pull pre-built images from GHCR
docker compose up -d

# Local development: Build images locally
docker compose -f docker-compose.yml -f docker-compose.local.yml up --build

# With GPU support for identifier service
docker compose -f docker-compose.yml -f docker-compose.gpu.yml up -d
```

Services will be available at:
- App (Frontend + API): http://localhost:3080
- Identifier: http://localhost:8099
- Prometheus Metrics: http://localhost:3080/metrics

### Docker Images

Images are automatically built and pushed to GitHub Container Registry on each commit to main:

| Image | Description |
|-------|-------------|
| `ghcr.io/codyseavey/tcg-tracker/app` | Combined Go API + Vue.js frontend |
| `ghcr.io/codyseavey/tcg-tracker/identifier` | Python OCR service |

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
go run cmd/server/main.go
```

The API will start on `http://localhost:8080`.

Environment variables:
- `PORT` - Server port (default: 8080)
- `DB_PATH` - SQLite database path (default: ./tcg_tracker.db)
- `POKEMON_DATA_DIR` - Pokemon TCG data directory
- `IDENTIFIER_SERVICE_URL` - Identifier service URL (default: http://127.0.0.1:8099)
- `ADMIN_KEY` - Admin key for collection modification (optional, auth disabled if not set)
- `JUSTTCG_API_KEY` - JustTCG API key for condition-based pricing
- `JUSTTCG_DAILY_LIMIT` - Daily API request limit (default: 1000)
- `SYNC_TCGPLAYER_IDS_ON_STARTUP` - Set to "true" to sync missing Pokemon TCGPlayerIDs on startup

#### 2. Frontend (Vue.js Web App)

```bash
cd frontend
npm install
npm run dev
```

The web app will start on `http://localhost:5173`.

#### 3. Identifier Service (Python OCR)

```bash
cd identifier
python -m venv venv && source venv/bin/activate
pip install -r requirements.txt

# Start server
uvicorn identifier.app:app --host 127.0.0.1 --port 8099
```

Environment variables:
- `HOST` - Bind address (default: 127.0.0.1)
- `PORT` - Server port (default: 8099)
- `USE_GPU` - Enable GPU acceleration (default: 1)

#### 4. Mobile App (Flutter)

```bash
cd mobile
flutter pub get
flutter run
```

Configure the server URL in settings to point to your backend IP.

## API Endpoints

### Cards
- `GET /api/cards/search?q={query}&game={mtg|pokemon}` - Search for cards
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
- `PUT /api/collection/:id` - Update collection item with smart split/merge (🔒)
- `DELETE /api/collection/:id` - Remove from collection (🔒)
- `GET /api/collection/stats` - Get collection statistics
- `GET /api/collection/stats/history` - Get historical collection value snapshots (for charting)
- `POST /api/collection/refresh-prices` - Immediately refresh prices for collection cards (🔒)

### Prices
- `GET /api/prices/status` - Get pricing quota status and next update time

### Admin (🔒)
- `POST /api/admin/sync-tcgplayer-ids` - Start async TCGPlayerID sync for collection cards
- `POST /api/admin/sync-tcgplayer-ids/blocking` - Sync TCGPlayerIDs and wait for completion
- `POST /api/admin/sync-tcgplayer-ids/set/:setName` - Sync TCGPlayerIDs for a specific set
- `GET /api/admin/sync-tcgplayer-ids/status` - Check sync status and quota

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

## External APIs

### Scryfall (MTG)
- No API key required
- Rate limit: 10 requests/second
- Documentation: https://scryfall.com/docs/api

### JustTCG (Pricing)
- Provides condition-specific pricing for Pokemon cards
- API key required
- Daily limit configurable via `JUSTTCG_DAILY_LIMIT` (free tier 100/day, paid tier 1000/day)
- Batch pricing uses TCGPlayerIDs for up to 100 cards per request

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

### Grafana Dashboard
Import the pre-built dashboard from `monitoring/grafana-dashboard.json` to visualize:
- Collection overview (card count, total value, breakdown by game)
- Price update status (queue size, update rate, batch duration)
- JustTCG API quota usage
- HTTP traffic and latency
