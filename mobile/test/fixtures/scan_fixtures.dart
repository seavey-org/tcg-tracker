import 'package:mobile/models/card.dart';
import 'card_fixtures.dart';

/// Sample scan metadata and scan result JSON for testing
class ScanFixtures {
  /// Complete scan metadata JSON with all fields
  static const Map<String, dynamic> completeScanMetadataJson = {
    'card_name': 'Charizard VMAX',
    'card_number': '025',
    'set_total': '185',
    'set_code': 'swsh4',
    'set_name': 'Vivid Voltage',
    'hp': '330',
    'rarity': 'Secret Rare',
    'is_foil': true,
    'foil_indicators': ['HOLO', 'SHINY'],
    'confidence': 0.85,
    'condition_hints': ['Light scratches', 'Minor wear'],
  };

  /// Minimal scan metadata JSON (empty/defaults)
  static const Map<String, dynamic> minimalScanMetadataJson = {};

  /// Scan metadata with low confidence
  static const Map<String, dynamic> lowConfidenceMetadataJson = {
    'card_name': 'Unknown Card',
    'confidence': 0.3,
  };

  /// Scan metadata with medium confidence
  static const Map<String, dynamic> mediumConfidenceMetadataJson = {
    'card_name': 'Some Card',
    'confidence': 0.55,
  };

  /// Scan metadata with high confidence
  static const Map<String, dynamic> highConfidenceMetadataJson = {
    'card_name': 'Known Card',
    'confidence': 0.75,
  };

  /// Non-foil scan metadata
  static const Map<String, dynamic> nonFoilMetadataJson = {
    'card_name': 'Regular Card',
    'is_foil': false,
    'foil_indicators': [],
    'confidence': 0.7,
  };

  /// Foil scan metadata
  static const Map<String, dynamic> foilMetadataJson = {
    'card_name': 'Shiny Card',
    'is_foil': true,
    'foil_indicators': ['HOLO', 'REVERSE'],
    'confidence': 0.8,
  };

  /// Complete scan result JSON
  static Map<String, dynamic> get completeScanResultJson => {
    'cards': [CardFixtures.completeCardJson, CardFixtures.mtgCardJson],
    'total_count': 2,
    'has_more': false,
    'parsed': completeScanMetadataJson,
  };

  /// Empty scan result JSON
  static Map<String, dynamic> get emptyScanResultJson => {
    'cards': [],
    'total_count': 0,
    'has_more': false,
    'parsed': minimalScanMetadataJson,
  };

  /// Scan result with foil detected
  static Map<String, dynamic> get foilScanResultJson => {
    'cards': [CardFixtures.completeCardJson],
    'total_count': 1,
    'has_more': false,
    'parsed': foilMetadataJson,
  };

  /// ScanMetadata instances
  static ScanMetadata get completeScanMetadata =>
      ScanMetadata.fromJson(completeScanMetadataJson);
  static ScanMetadata get minimalScanMetadata =>
      ScanMetadata.fromJson(minimalScanMetadataJson);
  static ScanMetadata get lowConfidenceMetadata =>
      ScanMetadata.fromJson(lowConfidenceMetadataJson);
  static ScanMetadata get mediumConfidenceMetadata =>
      ScanMetadata.fromJson(mediumConfidenceMetadataJson);
  static ScanMetadata get highConfidenceMetadata =>
      ScanMetadata.fromJson(highConfidenceMetadataJson);
  static ScanMetadata get nonFoilMetadata =>
      ScanMetadata.fromJson(nonFoilMetadataJson);
  static ScanMetadata get foilMetadata =>
      ScanMetadata.fromJson(foilMetadataJson);

  /// ScanResult instances
  static ScanResult get completeScanResult =>
      ScanResult.fromJson(completeScanResultJson);
  static ScanResult get emptyScanResult =>
      ScanResult.fromJson(emptyScanResultJson);
  static ScanResult get foilScanResult =>
      ScanResult.fromJson(foilScanResultJson);
}
