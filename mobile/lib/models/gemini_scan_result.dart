import 'card.dart';

/// Result from Gemini-based card identification
/// This is the new contract for /api/cards/identify-image
class GeminiScanResult {
  /// Matched card ID (empty if no confident match)
  final String cardId;

  /// Card name as it appeared on the scanned card (may be non-English)
  final String cardName;

  /// English canonical name for lookup/display (always English)
  final String canonicalNameEN;

  /// Set code (e.g., "swsh4", "2XM")
  final String setCode;

  /// Set name (e.g., "Vivid Voltage", "Double Masters")
  final String setName;

  /// Collector number
  final String cardNumber;

  /// Game type: "pokemon", "mtg", or "unknown"
  final String game;

  /// Language observed on the physical card (e.g., "Japanese", "English", "German")
  final String observedLanguage;

  /// Confidence score 0.0-1.0
  final double confidence;

  /// Gemini's reasoning for the identification
  final String reasoning;

  /// Number of API turns used
  final int turnsUsed;

  /// Resolved card objects for user selection
  /// First card is the best match (if cardId is set), followed by alternatives
  final List<CardModel> cards;

  GeminiScanResult({
    required this.cardId,
    required this.cardName,
    required this.canonicalNameEN,
    required this.setCode,
    required this.setName,
    required this.cardNumber,
    required this.game,
    required this.observedLanguage,
    required this.confidence,
    required this.reasoning,
    required this.turnsUsed,
    required this.cards,
  });

  factory GeminiScanResult.fromJson(Map<String, dynamic> json) {
    return GeminiScanResult(
      cardId: json['card_id'] ?? '',
      cardName: json['card_name'] ?? '',
      canonicalNameEN: json['canonical_name_en'] ?? json['card_name'] ?? '',
      setCode: json['set_code'] ?? '',
      setName: json['set_name'] ?? '',
      cardNumber: json['card_number'] ?? '',
      game: json['game'] ?? 'unknown',
      observedLanguage: json['observed_language'] ?? 'English',
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0.0,
      reasoning: json['reasoning'] ?? '',
      turnsUsed: json['turns_used'] ?? 0,
      cards:
          (json['cards'] as List<dynamic>?)
              ?.map((c) => CardModel.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  /// Returns true if Gemini found a confident match
  bool get hasConfidentMatch => cardId.isNotEmpty && confidence >= 0.7;

  /// Returns true if this is a non-English card
  bool get isNonEnglish =>
      observedLanguage.isNotEmpty &&
      observedLanguage.toLowerCase() != 'english';

  /// Returns a human-readable confidence label
  String get confidenceLabel {
    if (confidence >= 0.9) return 'Very High';
    if (confidence >= 0.7) return 'High';
    if (confidence >= 0.5) return 'Medium';
    if (confidence >= 0.3) return 'Low';
    return 'Very Low';
  }

  /// Returns the best card match, or null if no cards
  CardModel? get bestMatch => cards.isNotEmpty ? cards.first : null;

  /// Returns true if there are multiple cards to choose from
  bool get hasMultipleChoices => cards.length > 1;

  /// For MTG cards: group cards by set for 2-phase selection
  /// Returns a map of setCode -> list of cards in that set
  Map<String, List<CardModel>> groupCardsBySet() {
    final grouped = <String, List<CardModel>>{};
    for (final card in cards) {
      final setCode = card.setCode ?? 'unknown';
      grouped.putIfAbsent(setCode, () => []).add(card);
    }
    return grouped;
  }

  /// For MTG cards: get unique sets with their names, sorted by card count
  List<MTGSetInfo> getMTGSets() {
    final grouped = groupCardsBySet();
    final sets = <MTGSetInfo>[];

    for (final entry in grouped.entries) {
      final cardsInSet = entry.value;
      if (cardsInSet.isNotEmpty) {
        sets.add(
          MTGSetInfo(
            setCode: entry.key,
            setName: cardsInSet.first.setName ?? entry.key,
            variantCount: cardsInSet.length,
            releaseYear: _extractYear(cardsInSet.first.releasedAt),
            // Best match indicator: if the primary cardId is in this set
            isBestMatch: cardsInSet.any((c) => c.id == cardId),
          ),
        );
      }
    }

    // Sort: best match first, then by variant count (descending)
    sets.sort((a, b) {
      if (a.isBestMatch && !b.isBestMatch) return -1;
      if (!a.isBestMatch && b.isBestMatch) return 1;
      return b.variantCount.compareTo(a.variantCount);
    });

    return sets;
  }

  String? _extractYear(String? releasedAt) {
    if (releasedAt == null || releasedAt.length < 4) return null;
    return releasedAt.substring(0, 4);
  }
}

/// Info about an MTG set for 2-phase selection
class MTGSetInfo {
  final String setCode;
  final String setName;
  final int variantCount;
  final String? releaseYear;
  final bool isBestMatch;

  MTGSetInfo({
    required this.setCode,
    required this.setName,
    required this.variantCount,
    this.releaseYear,
    this.isBestMatch = false,
  });

  String get variantCountLabel =>
      variantCount == 1 ? '1 variant' : '$variantCount variants';
}
