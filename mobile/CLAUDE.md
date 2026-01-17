# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
flutter pub get                      # Install dependencies
flutter run                          # Run on connected device/emulator
flutter test                         # Run all tests (113 tests)
flutter test test/unit/              # Run unit tests only
flutter test test/widget/            # Run widget tests only
flutter test --plain-name "test name" path/to/test.dart  # Run single test
flutter analyze                      # Run linter
flutter build apk                    # Build Android APK
flutter build ios                    # Build iOS app
```

## Architecture

```
lib/
├── main.dart              # App entry, Material3 theme, route setup
├── models/
│   └── card.dart          # CardModel, CardSearchResult, ScanMetadata, ScanResult
├── screens/
│   ├── camera_screen.dart      # Camera capture + OCR, home screen
│   ├── scan_result_screen.dart # Display results, add to collection
│   └── settings_screen.dart    # Server URL configuration
└── services/
    ├── api_service.dart        # HTTP client, backend communication
    ├── camera_service.dart     # Camera abstraction (testable)
    └── ocr_service.dart        # ML Kit OCR abstraction (testable)
```

**Service layer uses constructor injection** for testability - all services accept optional dependencies that default to real implementations.

## Key Data Flow

1. **CameraScreen** captures image → ML Kit extracts text lines → sends to backend `/api/cards/identify`
2. Backend's OCRParser extracts metadata (name, set code, card number, foil indicators, confidence)
3. Backend searches and returns matching cards with parsed metadata
4. **ScanResultScreen** displays results with confidence badge, allows adding to collection
5. Collection additions POST to `/api/collection` with quantity, condition, foil status

## Testing

Tests use `mocktail` for mocking and `network_image_mock` for Image.network widgets.

```
test/
├── fixtures/           # Sample JSON data (CardFixtures, ScanFixtures)
├── helpers/            # Test utilities (SharedPreferences setup, widget wrappers)
├── mocks/              # MockApiService, MockHttpClient
├── unit/models/        # Model fromJson parsing tests
├── unit/services/      # ApiService tests with mocked HTTP
└── widget/screens/     # Screen widget tests
```

**SharedPreferences mocking**: Use `SharedPreferences.setMockInitialValues({'server_url': 'http://test:8080'})` before tests.

## Models

| Class | Purpose |
|-------|---------|
| `CardModel` | Single card with id, name, setName, setCode, cardNumber, rarity, imageUrl, priceUsd, priceFoilUsd |
| `CardSearchResult` | Search response: cards list, totalCount, hasMore |
| `ScanMetadata` | OCR parsing results: cardName, cardNumber, setCode, confidence, isFoil, foilIndicators |
| `ScanResult` | Combined: cards + metadata from `/cards/identify` |

Helper getters: `CardModel.displayPrice` returns formatted price or "N/A", `CardModel.displaySet` returns setName → setCode → "Unknown Set" fallback.

## Backend Integration

- Server URL stored in secure storage (default: `https://tcg.seavey.dev`)
- 35-second timeout on all HTTP requests
- ApiService methods: `searchCards(query, game)`, `identifyCard(text, game)`, `addToCollection(cardId, quantity, condition, foil)`

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `camera` | Camera capture |
| `google_mlkit_text_recognition` | OCR text extraction |
| `http` | HTTP client |
| `shared_preferences` | Local storage |
| `permission_handler` | Camera permissions |
| `mocktail` | Test mocking |
| `network_image_mock` | Mock network images in tests |
