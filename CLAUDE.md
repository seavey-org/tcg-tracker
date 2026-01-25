# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TCG Tracker is a trading card game collection management application supporting Pokemon and Magic: The Gathering cards. It features card scanning via mobile camera with Gemini AI-powered identification, price tracking with caching, and collection management.

## Architecture

```
tcg-tracker/
├── backend/          # Go REST API server with Gemini integration
├── frontend/         # Vue.js 3 web application
└── mobile/           # Flutter mobile app (camera + image upload)
```

## Tech Stack

- **Backend**: Go 1.24+, Gin framework, GORM, SQLite, Gemini API (function calling)
- **Frontend**: Vue.js 3, Vite, Pinia, Vue Router, Tailwind CSS
- **Mobile**: Flutter, camera integration (no client-side OCR)

## Build & Run Commands

### Docker Compose (Recommended)
```bash
# Production: Pull pre-built images from GHCR
docker compose up -d

# Local development: Build images locally
docker compose -f docker-compose.yml -f docker-compose.local.yml up --build

# Deploy specific version (for rollback)
IMAGE_TAG=abc123def docker compose up -d
```

### CI/CD & Docker Images
- Images are built and pushed to GHCR on each commit to main
- Images: `ghcr.io/codyseavey/tcg-tracker/app`
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
- `GOOGLE_API_KEY` - Gemini API key for card identification (**required** for scanning)

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

## Key Backend Services

| Service | File | Purpose |
|---------|------|---------|
| `GeminiService` | `internal/services/gemini_service.go` | Card identification via Gemini with function calling |
| `PokemonHybridService` | `internal/services/pokemon_hybrid.go` | Pokemon card search with local data |
| `ScryfallService` | `internal/services/scryfall.go` | MTG card search via Scryfall API |
| `JustTCGService` | `internal/services/justtcg.go` | Condition-based pricing from JustTCG API (sole price source) |
| `PriceService` | `internal/services/price_service.go` | Unified price fetching from JustTCG |
| `PriceWorker` | `internal/services/price_worker.go` | Background price updates with priority queue (user requests, then collection) |
| `AdminKeyAuth` | `internal/middleware/auth.go` | Admin key authentication middleware |
| `SnapshotService` | `internal/services/snapshot_service.go` | Daily collection value snapshots for historical tracking |
| `ImageStorageService` | `internal/services/image_storage.go` | Store and retrieve scanned card images |
| `TCGPlayerSyncService` | `internal/services/tcgplayer_sync.go` | Bulk sync TCGPlayerIDs from JustTCG for Pokemon cards |

## API Endpoints

Base URL: `http://localhost:8080/api`

### Cards
- `GET /cards/search?q=<query>&game=<pokemon|mtg>` - Search cards
- `GET /cards/:id` - Get card by ID
- `GET /cards/:id/prices` - Get condition-specific prices for a card
- `POST /cards/identify-image` - Identify card from uploaded image (Gemini-powered)
- `POST /cards/:id/refresh-price` - Refresh single card price

### Auth
- `GET /auth/status` - Check if authentication is enabled
- `POST /auth/verify` - Verify admin key (returns valid: true/false)

### Collection
- `GET /collection` - Get user's collection (flat list)
- `GET /collection/grouped?q=<search>&game=<pokemon|mtg>&sort=<added_at|name|price_updated>` - Get collection grouped by card_id with variants summary (supports search and sorting)
- `POST /collection` - Add card to collection (requires admin key)
- `PUT /collection/:id` - Update collection item with smart split/merge (requires admin key)
- `DELETE /collection/:id` - Remove from collection (requires admin key)
- `GET /collection/stats` - Get collection statistics
- `GET /collection/stats/history` - Get historical collection value snapshots (for charting)
- `POST /collection/refresh-prices` - Trigger immediate price update batch (up to 100 cards) (requires admin key)

### Prices
- `GET /prices/status` - Get API quota status and next update time

### Admin (requires admin key)
- `POST /admin/sync-tcgplayer-ids` - Start async TCGPlayerID sync for collection cards
- `POST /admin/sync-tcgplayer-ids/blocking` - Sync TCGPlayerIDs and wait for completion
- `POST /admin/sync-tcgplayer-ids/set/:setName` - Sync TCGPlayerIDs for a specific set
- `GET /admin/sync-tcgplayer-ids/status` - Check sync status and quota

### Health & Monitoring
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics endpoint (for Grafana)

## Data Flow

```
Mobile App: Capture image → POST /api/cards/identify-image
                                    │
                                    ▼
                          GeminiService.IdentifyCard()
                                    │
                                    ▼
                     Gemini analyzes image, calls tools:
                     ├─ search_pokemon_cards("Charizard")
                     ├─ view_card_image("swsh4-25", "pokemon")
                     └─ Returns best match with confidence
                                    │
                                    ▼
                          Return card to client
```

1. **Card Search**: User searches → Backend queries local Pokemon data or Scryfall API → Returns cards with cached prices
2. **Card Scanning**: Mobile captures image → Backend sends to Gemini → Gemini uses function calling to search cards and view images → Returns identified card
3. **Price Updates**: Background worker runs every 15 minutes → Updates up to 100 cards per batch via JustTCG (skips when quota exhausted)

## Gemini Card Identification

The `GeminiService` uses Gemini's function calling capability to identify cards from images:

### How It Works

1. **Image Analysis**: Gemini receives the card image and analyzes visual features (artwork, text, layout, symbols)
2. **Function Calling**: Gemini can call these tools during identification:
   - **Search tools** (return rich metadata for filtering):
     - `search_pokemon_cards(name)` - Returns hp, types, subtypes, regulation_mark, artist, release_date
     - `search_mtg_cards(name)` - Returns type_line, mana_cost, border_color, frame_effects, artist
     - `search_japanese_pokemon_cards(name)` - For Japanese-exclusive artwork
     - `search_cards_in_set(set_code, name?, game)` - Targeted search within a specific set
   - **Lookup tools**:
     - `get_pokemon_card(set_code, number)` / `get_mtg_card(set_code, number)` - Get exact card
     - `list_pokemon_sets(query)` / `list_mtg_sets(query)` - Find set codes by name
     - `get_set_info(set_code, game)` - Get set details with symbol description
   - **Verification tools**:
     - `view_card_image(card_id, game)` - View single card image for comparison
     - `view_multiple_card_images(card_ids, game)` - Batch view 2-3 images at once
     - `get_card_details(card_id, game)` - Get full card data (attacks, abilities, text)
3. **Multi-turn Conversation**: Gemini iterates, calling tools as needed until confident
4. **Result**: Returns the matched card ID with confidence score

### Key Benefits
- **No OCR Required**: Gemini identifies cards visually, bypassing OCR errors
- **Language Detection**: Gemini detects the card's printed language (Japanese, German, French, etc.)
- **English Canonical Names**: Always returns the English name for lookup, even for non-English cards
- **Self-correcting**: Gemini can compare card images to verify matches
- **Holistic Analysis**: Uses artwork, layout, set symbols, not just text

### Response Contract
The `/api/cards/identify-image` endpoint returns:
```json
{
  "card_id": "swsh4-025",
  "card_name": "リザードンVMAX",
  "canonical_name_en": "Charizard VMAX",
  "set_code": "swsh4",
  "set_name": "Vivid Voltage",
  "card_number": "025",
  "game": "pokemon",
  "observed_language": "Japanese",
  "confidence": 0.95,
  "reasoning": "Matched by artwork comparison...",
  "turns_used": 4,
  "cards": [{ ... full Card objects for selection ... }]
}
```

Key fields:
- `observed_language`: Language detected on the physical card (used as default for collection)
- `canonical_name_en`: English name (used for display and search)
- `cards`: Always populated with resolved Card objects for user selection

### Configuration
- Requires `GOOGLE_API_KEY` environment variable
- Model: `gemini-2.0-flash` (configurable in `gemini_service.go`)
- Max 10 tool call iterations per identification

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
- Condition, printing, and language-specific prices (NM, LP, MP, HP, DMG) stored in `card_prices` table
- Prices keyed by card_id + condition + printing + language (English, Japanese, German, French, Italian)
- Base prices (NM only) kept in `cards` table for backward compatibility
- All prices come from JustTCG API (no fallback to other sources for uniform data)
- `PriceService` returns cached data only (no live API calls)
- `Card.GetPrice()` fallback chain (handles holo-only cards, WotC-era cards, and foreign cards):
  1. Exact condition+printing+language match from `card_prices`
  2. NM price for same printing and language (if condition is not NM)
  3. Printing-specific fallback within same language:
     - Foil -> Normal (for holo-only cards where JustTCG stores price as "Normal")
     - Reverse Holo -> Normal (parallel version of Normal card, NOT related to Holo Rare)
     - Normal -> Unlimited (for WotC-era cards)
     - Unlimited -> Normal (for modern cards)
     - 1st Edition -> Unlimited -> Normal (WotC-era, different print run, NOT a foil)
  4. If non-English language has no price, fall back to English prices
  5. Base prices (`PriceFoilUSD` for foil variants, `PriceUSD` otherwise)
  6. Final cross-fallback if primary base price is zero
- `Card.GetPriceWithSource()` returns price with metadata:
  - `PriceLanguage`: Which language's price was actually used
  - `IsFallback`: True if price is from a different language than requested
  - Collection variants include `price_language` and `price_fallback` fields to indicate when Japanese cards are priced using English market data
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
- `POST /collection/refresh-prices` triggers an immediate batch update (doesn't wait for 15-minute interval)
- Updates up to 100 cards using the same priority logic as the background worker
- If fewer than 100 cards are queued/need prices, fills batch with cards having oldest prices
- Still respects rate limits and daily quota

Worker skips updates when daily quota is exhausted (resets at midnight).
Collection stats use condition and printing-appropriate prices.

### CardSearcher Interface
Both `PokemonHybridService` and `ScryfallService` implement the `CardSearcher` interface used by `GeminiService`:

```go
type CardSearcher interface {
    SearchByName(ctx, name, limit) ([]CandidateCard, error)
    SearchInSet(ctx, setCode, name, limit) ([]CandidateCard, error)
    GetBySetAndNumber(ctx, setCode, number) (*CandidateCard, error)
    GetCardImage(ctx, cardID) (base64String, error)
    GetCardDetails(ctx, cardID) (*CardDetails, error)
    ListSets(ctx, query) ([]SetInfo, error)
    GetSetInfo(ctx, setCode) (*SetDetails, error)
}
```

This allows Gemini to search for cards, fetch images, and get detailed card data for verification.

### LRU Image Cache
Card images fetched by Gemini during identification are cached to avoid re-downloading:
- **Size**: 50 images max (LRU eviction)
- **Memory**: ~5-10MB (base64-encoded images)
- **Scope**: In-memory, per-instance (lost on restart)
- **Purpose**: Speeds up multi-turn identification when Gemini views the same card multiple times

### MTG Sets Cache
The `ScryfallService` caches the list of all MTG sets (from Scryfall API):
- **TTL**: 24 hours
- **Purpose**: Avoids fetching ~700 sets on every `list_mtg_sets` call
- **Size**: ~100KB

### Inverted Index for Card Matching
The `PokemonHybridService` uses an inverted index for fast full-text card matching:

**Index Structure:**
- `wordIndex map[string][]int` - maps words to card indices for full-text search
- `idIndex map[string]int` - maps card ID to card index for O(1) lookups
- Built at startup from ~20,000 cards
- ~13,000 unique indexed words
- ~20MB additional memory

**Performance:**
| Scenario | Time | Notes |
|----------|------|-------|
| Good match (index hit) | ~1.5ms | 18x faster than full scan |
| With set filter | ~37μs | 720x faster than full scan |
| Poor match (fallback) | ~27ms | Falls back to full scan |

### MTG 2-Phase Card Selection
MTG cards often have many printings across different sets. The 2-phase selection UI helps users pick the exact printing:

**How it works:**
1. Scan returns a `cards` array containing all candidate printings
2. **Phase 1**: Mobile groups cards by set client-side, user sees list of sets
3. **Phase 2**: User taps a set to see all variants (foil, showcase, borderless, etc.)
4. User selects the exact variant to add to collection

**Client-side grouping (mobile):**
- `GeminiScanResult.groupCardsBySet()` - Groups flat card list by set code
- `GeminiScanResult.getMTGSets()` - Returns `MTGSetInfo` objects sorted by:
  - Best match first (set containing the identified card_id)
  - Then by variant count (descending)
- `CardModel.variantLabel` computes human-readable variant labels (e.g., "Borderless Foil")

**Backend provides:**
- Card model includes variant info: `Finishes`, `FrameEffects`, `PromoTypes`, `ReleasedAt`
- Flat list of all candidate cards in `cards` array

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
- Non-scanned cards merge into existing stacks with matching card_id+condition+printing+language

### Frontend Serving
The Go backend can serve the Vue.js frontend from `FRONTEND_DIST_PATH`:
- Static assets from `/assets`
- SPA fallback routes non-API paths to `index.html`

## Database

SQLite database with GORM models in `internal/models/`:
- `Card` - Card data with prices, images, metadata
- `CardPrice` - Condition, printing, and language-specific prices (NM, LP, MP, HP, DMG) for each card/printing/language combo
- `CollectionItem` - User's collection entries with quantity, condition, printing type, and language

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
