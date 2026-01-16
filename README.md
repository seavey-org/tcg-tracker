# TCG Tracker

A trading card collection tracker for Magic: The Gathering and Pokemon cards with camera-based card scanning, automatic identification, and price tracking.

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
    ┌────────────────┼────────────────┐
    │                │                │
┌───▼───┐      ┌─────▼─────┐    ┌─────▼─────┐
│SQLite │      │ Scryfall  │    │Pokemon TCG│
│  DB   │      │   API     │    │    API    │
└───────┘      └───────────┘    └───────────┘
```

## Features

- **Card Search**: Search for MTG and Pokemon cards using external APIs
- **Collection Management**: Add, update, and remove cards from your collection
- **Price Tracking**: View current market prices with refresh capability
- **Mobile Scanning**: Use your phone camera to scan and identify cards via OCR
- **Dashboard**: View collection statistics and total value

## Prerequisites

- **Go** 1.21+ (for backend)
- **Node.js** 18+ (for frontend)
- **Flutter** 3.0+ (for mobile app)

## Quick Start

### 1. Backend (Go API)

```bash
cd backend

# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go
```

The API will start on `http://localhost:8080`.

Optional environment variables:
- `PORT` - Server port (default: 8080)
- `DB_PATH` - SQLite database path (default: ./tcg_tracker.db)
- `POKEMON_TCG_API_KEY` - Pokemon TCG API key (optional, increases rate limits)

### 2. Frontend (Vue.js Web App)

```bash
cd frontend

# Install dependencies
npm install

# Run development server
npm run dev
```

The web app will start on `http://localhost:5173`.

To build for production:
```bash
npm run build
```

### 3. Mobile App (Flutter)

```bash
cd mobile

# Get dependencies
flutter pub get

# Run on connected device/emulator
flutter run
```

**Important**: Configure the server URL in the mobile app settings to point to your backend's IP address (e.g., `http://192.168.1.100:8080`).

## API Endpoints

### Cards
- `GET /api/cards/search?q={query}&game={mtg|pokemon}` - Search for cards
- `GET /api/cards/:id?game={mtg|pokemon}` - Get card details
- `POST /api/cards/identify` - Identify card from OCR text

### Collection
- `GET /api/collection` - Get all collection items
- `POST /api/collection` - Add card to collection
- `PUT /api/collection/:id` - Update collection item
- `DELETE /api/collection/:id` - Remove from collection
- `GET /api/collection/stats` - Get collection statistics
- `POST /api/collection/refresh-prices` - Refresh all prices

## Project Structure

```
tcg-tracker/
├── backend/                 # Go API server
│   ├── cmd/server/          # Main entry point
│   └── internal/
│       ├── api/             # HTTP handlers and routes
│       ├── database/        # SQLite setup
│       ├── models/          # Data models
│       └── services/        # External API services
├── frontend/                # Vue.js web application
│   └── src/
│       ├── components/      # Reusable Vue components
│       ├── views/           # Page components
│       ├── services/        # API client
│       └── stores/          # Pinia state management
└── mobile/                  # Flutter mobile app
    └── lib/
        ├── models/          # Data models
        ├── screens/         # App screens
        └── services/        # API and OCR services
```

## External APIs

### Scryfall (MTG)
- No API key required
- Rate limit: 10 requests/second
- Documentation: https://scryfall.com/docs/api

### Pokemon TCG API
- API key recommended for higher rate limits
- Free tier: 1000 requests/day
- Get API key: https://dev.pokemontcg.io/
