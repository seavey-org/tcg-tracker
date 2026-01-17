import 'package:flutter_test/flutter_test.dart';
import 'package:mobile/models/card.dart';
import '../../fixtures/card_fixtures.dart';

void main() {
  group('CardModel', () {
    group('fromJson', () {
      test('parses complete card JSON with all fields', () {
        final card = CardModel.fromJson(CardFixtures.completeCardJson);

        expect(card.id, 'swsh4-025');
        expect(card.game, 'pokemon');
        expect(card.name, 'Charizard VMAX');
        expect(card.setName, 'Vivid Voltage');
        expect(card.setCode, 'swsh4');
        expect(card.cardNumber, '025');
        expect(card.rarity, 'Secret Rare');
        expect(card.imageUrl, 'https://images.pokemontcg.io/swsh4/025.png');
        expect(card.priceUsd, 125.50);
        expect(card.priceFoilUsd, 200.00);
      });

      test('parses minimal card JSON with only required fields', () {
        final card = CardModel.fromJson(CardFixtures.minimalCardJson);

        expect(card.id, 'test-001');
        expect(card.game, 'mtg');
        expect(card.name, 'Test Card');
        expect(card.setName, isNull);
        expect(card.setCode, isNull);
        expect(card.cardNumber, isNull);
        expect(card.rarity, isNull);
        expect(card.imageUrl, isNull);
        expect(card.priceUsd, isNull);
        expect(card.priceFoilUsd, isNull);
      });

      test('handles null optional fields correctly', () {
        final card = CardModel.fromJson(CardFixtures.nullOptionalsJson);

        expect(card.id, 'null-test-001');
        expect(card.game, 'pokemon');
        expect(card.name, 'Nullable Card');
        expect(card.setName, isNull);
        expect(card.setCode, isNull);
        expect(card.cardNumber, isNull);
        expect(card.rarity, isNull);
        expect(card.imageUrl, isNull);
        expect(card.priceUsd, isNull);
        expect(card.priceFoilUsd, isNull);
      });

      test('parses integer price as double', () {
        final json = {
          'id': 'int-price',
          'game': 'mtg',
          'name': 'Integer Price Card',
          'price_usd': 10,
          'price_foil_usd': 20,
        };
        final card = CardModel.fromJson(json);

        expect(card.priceUsd, 10.0);
        expect(card.priceFoilUsd, 20.0);
      });

      test('handles missing id/game/name with empty string defaults', () {
        final card = CardModel.fromJson({});

        expect(card.id, '');
        expect(card.game, '');
        expect(card.name, '');
      });
    });

    group('displayPrice', () {
      test('returns formatted price for valid priceUsd', () {
        final card = CardFixtures.completeCard;
        expect(card.displayPrice, '\$125.50');
      });

      test('returns N/A when priceUsd is null', () {
        final card = CardFixtures.nullOptionalsCard;
        expect(card.displayPrice, 'N/A');
      });

      test('returns N/A when priceUsd is zero', () {
        final card = CardFixtures.zeroPriceCard;
        expect(card.displayPrice, 'N/A');
      });

      test('formats price with two decimal places', () {
        final card = CardModel(
          id: 'test',
          game: 'mtg',
          name: 'Test',
          priceUsd: 5.5,
        );
        expect(card.displayPrice, '\$5.50');
      });

      test('formats whole number price with decimals', () {
        final card = CardModel(
          id: 'test',
          game: 'mtg',
          name: 'Test',
          priceUsd: 100.0,
        );
        expect(card.displayPrice, '\$100.00');
      });
    });

    group('displaySet', () {
      test('returns setName when available', () {
        final card = CardFixtures.completeCard;
        expect(card.displaySet, 'Vivid Voltage');
      });

      test('returns setCode when setName is null', () {
        final card = CardFixtures.setCodeOnlyCard;
        expect(card.displaySet, 'swsh4');
      });

      test('returns Unknown Set when both setName and setCode are null', () {
        final card = CardFixtures.noSetCard;
        expect(card.displaySet, 'Unknown Set');
      });
    });
  });

  group('CardSearchResult', () {
    group('fromJson', () {
      test('parses complete search result', () {
        final result = CardSearchResult.fromJson(CardFixtures.searchResultJson);

        expect(result.cards.length, 2);
        expect(result.totalCount, 2);
        expect(result.hasMore, false);
        expect(result.cards[0].name, 'Charizard VMAX');
        expect(result.cards[1].name, 'Lightning Bolt');
      });

      test('parses empty search result', () {
        final result = CardSearchResult.fromJson(CardFixtures.emptySearchResultJson);

        expect(result.cards, isEmpty);
        expect(result.totalCount, 0);
        expect(result.hasMore, false);
      });

      test('handles missing fields with defaults', () {
        final result = CardSearchResult.fromJson({});

        expect(result.cards, isEmpty);
        expect(result.totalCount, 0);
        expect(result.hasMore, false);
      });

      test('handles null cards array', () {
        final result = CardSearchResult.fromJson({
          'cards': null,
          'total_count': 0,
          'has_more': false,
        });

        expect(result.cards, isEmpty);
      });

      test('parses hasMore true value', () {
        final result = CardSearchResult.fromJson({
          'cards': [],
          'total_count': 100,
          'has_more': true,
        });

        expect(result.hasMore, true);
        expect(result.totalCount, 100);
      });
    });
  });
}
