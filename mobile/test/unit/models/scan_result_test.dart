import 'package:flutter_test/flutter_test.dart';
import 'package:mobile/models/card.dart';
import '../../fixtures/card_fixtures.dart';
import '../../fixtures/scan_fixtures.dart';

void main() {
  group('ScanResult', () {
    group('fromJson', () {
      test('parses complete scan result with cards and metadata', () {
        final result = ScanResult.fromJson(ScanFixtures.completeScanResultJson);

        expect(result.cards.length, 2);
        expect(result.totalCount, 2);
        expect(result.hasMore, false);
        expect(result.cards[0].name, 'Charizard VMAX');
        expect(result.cards[1].name, 'Lightning Bolt');
        expect(result.metadata.cardName, 'Charizard VMAX');
        expect(result.metadata.confidence, 0.85);
      });

      test('parses empty scan result', () {
        final result = ScanResult.fromJson(ScanFixtures.emptyScanResultJson);

        expect(result.cards, isEmpty);
        expect(result.totalCount, 0);
        expect(result.hasMore, false);
        expect(result.metadata.cardName, isNull);
        expect(result.metadata.confidence, 0.0);
      });

      test('handles missing parsed field with empty metadata', () {
        final result = ScanResult.fromJson({
          'cards': [],
          'total_count': 0,
          'has_more': false,
        });

        expect(result.metadata.cardName, isNull);
        expect(result.metadata.confidence, 0.0);
        expect(result.metadata.isFoil, false);
      });

      test('handles null parsed field', () {
        final result = ScanResult.fromJson({
          'cards': [],
          'total_count': 0,
          'has_more': false,
          'parsed': null,
        });

        expect(result.metadata.cardName, isNull);
      });

      test('parses foil scan result correctly', () {
        final result = ScanResult.fromJson(ScanFixtures.foilScanResultJson);

        expect(result.cards.length, 1);
        expect(result.metadata.isFoil, true);
        expect(result.metadata.foilIndicators, contains('HOLO'));
      });

      test('handles null cards array', () {
        final result = ScanResult.fromJson({
          'cards': null,
          'total_count': 0,
          'has_more': false,
          'parsed': {},
        });

        expect(result.cards, isEmpty);
      });

      test('parses hasMore true value', () {
        final result = ScanResult.fromJson({
          'cards': [CardFixtures.completeCardJson],
          'total_count': 100,
          'has_more': true,
          'parsed': ScanFixtures.completeScanMetadataJson,
        });

        expect(result.hasMore, true);
        expect(result.totalCount, 100);
        expect(result.cards.length, 1);
      });

      test('handles missing fields with defaults', () {
        final result = ScanResult.fromJson({});

        expect(result.cards, isEmpty);
        expect(result.totalCount, 0);
        expect(result.hasMore, false);
      });
    });

    group('fixture instances', () {
      test('completeScanResult has correct structure', () {
        final result = ScanFixtures.completeScanResult;

        expect(result.cards.length, 2);
        expect(result.metadata.isFoil, true);
        expect(result.metadata.confidence, 0.85);
      });

      test('emptyScanResult has no cards', () {
        final result = ScanFixtures.emptyScanResult;

        expect(result.cards, isEmpty);
        expect(result.metadata.confidence, 0.0);
      });

      test('foilScanResult has foil metadata', () {
        final result = ScanFixtures.foilScanResult;

        expect(result.metadata.isFoil, true);
        expect(result.metadata.foilIndicators, isNotEmpty);
      });
    });
  });
}
