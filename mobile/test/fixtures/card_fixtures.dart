import 'package:mobile/models/card.dart';

/// Sample card JSON for testing
class CardFixtures {
  /// Complete Pokemon card JSON with all fields
  static const Map<String, dynamic> completeCardJson = {
    'id': 'swsh4-025',
    'game': 'pokemon',
    'name': 'Charizard VMAX',
    'set_name': 'Vivid Voltage',
    'set_code': 'swsh4',
    'card_number': '025',
    'rarity': 'Secret Rare',
    'image_url': 'https://images.pokemontcg.io/swsh4/025.png',
    'price_usd': 125.50,
    'price_foil_usd': 200.00,
  };

  /// Minimal card JSON with only required fields
  static const Map<String, dynamic> minimalCardJson = {
    'id': 'test-001',
    'game': 'mtg',
    'name': 'Test Card',
  };

  /// Card JSON with null optional fields
  static const Map<String, dynamic> nullOptionalsJson = {
    'id': 'null-test-001',
    'game': 'pokemon',
    'name': 'Nullable Card',
    'set_name': null,
    'set_code': null,
    'card_number': null,
    'rarity': null,
    'image_url': null,
    'price_usd': null,
    'price_foil_usd': null,
  };

  /// Card with zero price
  static const Map<String, dynamic> zeroPriceJson = {
    'id': 'zero-price-001',
    'game': 'mtg',
    'name': 'Free Card',
    'price_usd': 0.0,
  };

  /// MTG card JSON
  static const Map<String, dynamic> mtgCardJson = {
    'id': 'neo-123',
    'game': 'mtg',
    'name': 'Lightning Bolt',
    'set_name': 'Neon Dynasty',
    'set_code': 'neo',
    'card_number': '123',
    'rarity': 'Common',
    'image_url': 'https://cards.scryfall.io/normal/front/neo/123.jpg',
    'price_usd': 2.50,
    'price_foil_usd': 5.00,
  };

  /// Card search result JSON
  static const Map<String, dynamic> searchResultJson = {
    'cards': [completeCardJson, mtgCardJson],
    'total_count': 2,
    'has_more': false,
  };

  /// Empty search result JSON
  static const Map<String, dynamic> emptySearchResultJson = {
    'cards': [],
    'total_count': 0,
    'has_more': false,
  };

  /// Card model instances
  static CardModel get completeCard => CardModel.fromJson(completeCardJson);
  static CardModel get minimalCard => CardModel.fromJson(minimalCardJson);
  static CardModel get nullOptionalsCard => CardModel.fromJson(nullOptionalsJson);
  static CardModel get zeroPriceCard => CardModel.fromJson(zeroPriceJson);
  static CardModel get mtgCard => CardModel.fromJson(mtgCardJson);

  /// Card with only setCode (no setName)
  static CardModel get setCodeOnlyCard => CardModel(
    id: 'setcode-only',
    game: 'pokemon',
    name: 'SetCode Only Card',
    setCode: 'swsh4',
  );

  /// Card with neither setCode nor setName
  static CardModel get noSetCard => CardModel(
    id: 'no-set',
    game: 'pokemon',
    name: 'No Set Card',
  );

  /// List of sample cards for list tests
  static List<CardModel> get sampleCardList => [
    completeCard,
    mtgCard,
    minimalCard,
  ];
}
