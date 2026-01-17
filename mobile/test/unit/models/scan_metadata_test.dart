import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/material.dart';
import 'package:mobile/models/card.dart';
import '../../fixtures/scan_fixtures.dart';

void main() {
  group('ScanMetadata', () {
    group('fromJson', () {
      test('parses complete scan metadata JSON with all fields', () {
        final metadata = ScanMetadata.fromJson(ScanFixtures.completeScanMetadataJson);

        expect(metadata.cardName, 'Charizard VMAX');
        expect(metadata.cardNumber, '025');
        expect(metadata.setTotal, '185');
        expect(metadata.setCode, 'swsh4');
        expect(metadata.setName, 'Vivid Voltage');
        expect(metadata.hp, '330');
        expect(metadata.rarity, 'Secret Rare');
        expect(metadata.isFoil, true);
        expect(metadata.foilIndicators, ['HOLO', 'SHINY']);
        expect(metadata.confidence, 0.85);
        expect(metadata.conditionHints, ['Light scratches', 'Minor wear']);
      });

      test('uses default values for missing fields', () {
        final metadata = ScanMetadata.fromJson(ScanFixtures.minimalScanMetadataJson);

        expect(metadata.cardName, isNull);
        expect(metadata.cardNumber, isNull);
        expect(metadata.setTotal, isNull);
        expect(metadata.setCode, isNull);
        expect(metadata.setName, isNull);
        expect(metadata.hp, isNull);
        expect(metadata.rarity, isNull);
        expect(metadata.isFoil, false);
        expect(metadata.foilIndicators, isEmpty);
        expect(metadata.confidence, 0.0);
        expect(metadata.conditionHints, isEmpty);
      });

      test('handles null foil_indicators array', () {
        final metadata = ScanMetadata.fromJson({
          'card_name': 'Test Card',
          'foil_indicators': null,
        });

        expect(metadata.foilIndicators, isEmpty);
      });

      test('handles null condition_hints array', () {
        final metadata = ScanMetadata.fromJson({
          'card_name': 'Test Card',
          'condition_hints': null,
        });

        expect(metadata.conditionHints, isEmpty);
      });

      test('parses confidence as double from integer', () {
        final metadata = ScanMetadata.fromJson({
          'confidence': 1,
        });

        expect(metadata.confidence, 1.0);
      });
    });

    group('detectionSummary', () {
      test('returns full summary with all data', () {
        final metadata = ScanFixtures.completeScanMetadata;
        final summary = metadata.detectionSummary;

        expect(summary, contains('Name: Charizard VMAX'));
        expect(summary, contains('Set: Vivid Voltage'));
        expect(summary, contains('#025/185'));
        expect(summary, contains('Secret Rare'));
        expect(summary, contains('Foil detected'));
      });

      test('returns "No details detected" for empty metadata', () {
        final metadata = ScanFixtures.minimalScanMetadata;
        expect(metadata.detectionSummary, 'No details detected');
      });

      test('includes "Foil detected" when isFoil is true', () {
        final metadata = ScanFixtures.foilMetadata;
        expect(metadata.detectionSummary, contains('Foil detected'));
      });

      test('omits "Foil detected" when isFoil is false', () {
        final metadata = ScanFixtures.nonFoilMetadata;
        expect(metadata.detectionSummary, isNot(contains('Foil detected')));
      });

      test('uses setCode when setName is null', () {
        final metadata = ScanMetadata.fromJson({
          'set_code': 'swsh4',
          'confidence': 0.5,
        });
        expect(metadata.detectionSummary, contains('Set: swsh4'));
      });

      test('shows card number without set total', () {
        final metadata = ScanMetadata.fromJson({
          'card_number': '025',
          'confidence': 0.5,
        });
        expect(metadata.detectionSummary, contains('#025'));
        expect(metadata.detectionSummary, isNot(contains('/')));
      });

      test('shows card number with set total', () {
        final metadata = ScanMetadata.fromJson({
          'card_number': '025',
          'set_total': '185',
          'confidence': 0.5,
        });
        expect(metadata.detectionSummary, contains('#025/185'));
      });

      test('handles empty cardName string', () {
        final metadata = ScanMetadata.fromJson({
          'card_name': '',
        });
        expect(metadata.detectionSummary, 'No details detected');
      });

      test('handles empty setCode string', () {
        final metadata = ScanMetadata.fromJson({
          'set_code': '',
        });
        expect(metadata.detectionSummary, 'No details detected');
      });
    });

    group('confidence color thresholds', () {
      Color getConfidenceColor(double confidence) {
        if (confidence >= 0.7) return Colors.green;
        if (confidence >= 0.4) return Colors.orange;
        return Colors.red;
      }

      test('returns green for confidence >= 0.7', () {
        expect(getConfidenceColor(0.7), Colors.green);
        expect(getConfidenceColor(0.85), Colors.green);
        expect(getConfidenceColor(1.0), Colors.green);
      });

      test('returns orange for confidence between 0.4 and 0.7', () {
        expect(getConfidenceColor(0.4), Colors.orange);
        expect(getConfidenceColor(0.55), Colors.orange);
        expect(getConfidenceColor(0.69), Colors.orange);
      });

      test('returns red for confidence < 0.4', () {
        expect(getConfidenceColor(0.0), Colors.red);
        expect(getConfidenceColor(0.3), Colors.red);
        expect(getConfidenceColor(0.39), Colors.red);
      });

      test('high confidence metadata has >= 0.7', () {
        final metadata = ScanFixtures.highConfidenceMetadata;
        expect(metadata.confidence, greaterThanOrEqualTo(0.7));
      });

      test('medium confidence metadata is between 0.4 and 0.7', () {
        final metadata = ScanFixtures.mediumConfidenceMetadata;
        expect(metadata.confidence, greaterThanOrEqualTo(0.4));
        expect(metadata.confidence, lessThan(0.7));
      });

      test('low confidence metadata is < 0.4', () {
        final metadata = ScanFixtures.lowConfidenceMetadata;
        expect(metadata.confidence, lessThan(0.4));
      });
    });
  });
}
