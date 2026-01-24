import 'package:mobile/models/card.dart';
import 'package:mobile/models/gemini_scan_result.dart';
import 'card_fixtures.dart';

/// Sample scan metadata and scan result JSON for testing
class ScanFixtures {
  // ============================================
  // GeminiScanResult fixtures (new Gemini-first flow)
  // ============================================

  /// Complete Gemini scan result JSON
  static Map<String, dynamic> get completeGeminiResultJson => {
    'card_id': 'swsh4-025',
    'card_name': 'Charizard VMAX',
    'canonical_name_en': 'Charizard VMAX',
    'set_code': 'swsh4',
    'set_name': 'Vivid Voltage',
    'card_number': '025',
    'game': 'pokemon',
    'observed_language': 'English',
    'confidence': 0.85,
    'reasoning': 'Matched by set code and collector number, artwork verified.',
    'turns_used': 3,
    'cards': [CardFixtures.completeCardJson],
  };

  /// Gemini result with multiple candidates (low confidence)
  static Map<String, dynamic> get multiCandidateGeminiResultJson => {
    'card_id': 'swsh4-025',
    'card_name': 'Charizard VMAX',
    'canonical_name_en': 'Charizard VMAX',
    'set_code': 'swsh4',
    'set_name': 'Vivid Voltage',
    'card_number': '025',
    'game': 'pokemon',
    'observed_language': 'English',
    'confidence': 0.6,
    'reasoning':
        'Multiple printings found, artwork similar but not exact match.',
    'turns_used': 5,
    'cards': [CardFixtures.completeCardJson, CardFixtures.mtgCardJson],
  };

  /// Gemini result for Japanese card
  static Map<String, dynamic> get japaneseGeminiResultJson => {
    'card_id': 'swsh4-025',
    'card_name': 'リザードンVMAX', // Japanese name
    'canonical_name_en': 'Charizard VMAX',
    'set_code': 'swsh4',
    'set_name': 'Vivid Voltage',
    'card_number': '025',
    'game': 'pokemon',
    'observed_language': 'Japanese',
    'confidence': 0.9,
    'reasoning':
        'Japanese card identified by artwork match with English database.',
    'turns_used': 4,
    'cards': [CardFixtures.completeCardJson],
  };

  /// Gemini result with no match
  static Map<String, dynamic> get noMatchGeminiResultJson => {
    'card_id': '',
    'card_name': 'Unknown Card',
    'canonical_name_en': 'Unknown Card',
    'set_code': '',
    'set_name': '',
    'card_number': '',
    'game': 'unknown',
    'observed_language': 'English',
    'confidence': 0.1,
    'reasoning': 'Could not identify card from image.',
    'turns_used': 10,
    'cards': [],
  };

  /// MTG Gemini result with multiple sets (for 2-phase selection)
  static Map<String, dynamic> get mtgMultiSetGeminiResultJson => {
    'card_id': '12345-abc',
    'card_name': 'Lightning Bolt',
    'canonical_name_en': 'Lightning Bolt',
    'set_code': 'm21',
    'set_name': 'Core Set 2021',
    'card_number': '152',
    'game': 'mtg',
    'observed_language': 'English',
    'confidence': 0.75,
    'reasoning': 'Card has many printings across sets.',
    'turns_used': 4,
    'cards': [
      CardFixtures.mtgCardJson,
      {
        ...CardFixtures.mtgCardJson,
        'id': 'mtg-2xm-123',
        'set_code': '2xm',
        'set_name': 'Double Masters',
      },
      {
        ...CardFixtures.mtgCardJson,
        'id': 'mtg-m20-456',
        'set_code': 'm20',
        'set_name': 'Core Set 2020',
      },
    ],
  };

  /// GeminiScanResult instances
  static GeminiScanResult get completeGeminiResult =>
      GeminiScanResult.fromJson(completeGeminiResultJson);
  static GeminiScanResult get multiCandidateGeminiResult =>
      GeminiScanResult.fromJson(multiCandidateGeminiResultJson);
  static GeminiScanResult get japaneseGeminiResult =>
      GeminiScanResult.fromJson(japaneseGeminiResultJson);
  static GeminiScanResult get noMatchGeminiResult =>
      GeminiScanResult.fromJson(noMatchGeminiResultJson);
  static GeminiScanResult get mtgMultiSetGeminiResult =>
      GeminiScanResult.fromJson(mtgMultiSetGeminiResultJson);

  // ============================================
  // Legacy ScanMetadata fixtures (kept for backward compatibility)
  // ============================================
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

  /// Foil scan metadata (high confidence, should trigger "Detected")
  static const Map<String, dynamic> foilMetadataJson = {
    'card_name': 'Shiny Card',
    'is_foil': true,
    'foil_indicators': ['HOLO', 'REVERSE'],
    'confidence': 0.8,
    'foil_confidence':
        0.9, // High confidence triggers "Detected" label and auto-fills foil
  };

  /// Japanese card scan metadata (detected language should be passed to collection)
  static const Map<String, dynamic> japaneseScanMetadataJson = {
    'card_name': 'Pikachu V',
    'card_number': '025',
    'set_code': 'swsh4',
    'confidence': 0.85,
    'detected_language': 'Japanese',
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
  static ScanMetadata get japaneseScanMetadata =>
      ScanMetadata.fromJson(japaneseScanMetadataJson);

  /// ScanResult instances
  static ScanResult get completeScanResult =>
      ScanResult.fromJson(completeScanResultJson);
  static ScanResult get emptyScanResult =>
      ScanResult.fromJson(emptyScanResultJson);
  static ScanResult get foilScanResult =>
      ScanResult.fromJson(foilScanResultJson);
}
