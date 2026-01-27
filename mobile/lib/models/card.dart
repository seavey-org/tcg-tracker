import 'collection_item.dart' show PrintingType;
import 'mtg_grouped_result.dart';

class CardModel {
  final String id;
  final String game;
  final String name;
  final String? setName;
  final String? setCode;
  final String? cardNumber;
  final String? rarity;
  final String? imageUrl;
  final double? priceUsd;
  final double? priceFoilUsd;
  final String? tcgplayerId;

  // MTG variant info (from Scryfall)
  final List<String>? finishes; // nonfoil, foil, etched
  final List<String>? frameEffects; // showcase, borderless, extendedart
  final List<String>? promoTypes; // buyabox, prerelease
  final String? releasedAt;

  // Set symbol URL (from backend, populated for all cards)
  final String? setSymbolUrl;

  CardModel({
    required this.id,
    required this.game,
    required this.name,
    this.setName,
    this.setCode,
    this.cardNumber,
    this.rarity,
    this.imageUrl,
    this.priceUsd,
    this.priceFoilUsd,
    this.tcgplayerId,
    this.finishes,
    this.frameEffects,
    this.promoTypes,
    this.releasedAt,
    this.setSymbolUrl,
  });

  factory CardModel.fromJson(Map<String, dynamic> json) {
    return CardModel(
      id: json['id'] ?? '',
      game: json['game'] ?? '',
      name: json['name'] ?? '',
      setName: json['set_name'],
      setCode: json['set_code'],
      cardNumber: json['card_number'],
      rarity: json['rarity'],
      imageUrl: json['image_url'],
      priceUsd: (json['price_usd'] as num?)?.toDouble(),
      priceFoilUsd: (json['price_foil_usd'] as num?)?.toDouble(),
      tcgplayerId: json['tcgplayer_id'],
      finishes: (json['finishes'] as List<dynamic>?)?.cast<String>(),
      frameEffects: (json['frame_effects'] as List<dynamic>?)?.cast<String>(),
      promoTypes: (json['promo_types'] as List<dynamic>?)?.cast<String>(),
      releasedAt: json['released_at'],
      setSymbolUrl: json['set_symbol_url'],
    );
  }

  /// Returns the TCGPlayer URL for this card, or null if no tcgplayerId
  String? get tcgplayerUrl => tcgplayerId != null
      ? 'https://www.tcgplayer.com/product/$tcgplayerId'
      : null;

  String get displayPrice {
    if (priceUsd == null || priceUsd == 0) return 'N/A';
    return '\$${priceUsd!.toStringAsFixed(2)}';
  }

  String get displaySet => setName ?? setCode ?? 'Unknown Set';

  /// Returns a human-readable variant label for MTG cards
  /// e.g., "Foil", "Showcase", "Borderless Foil", "Etched Foil"
  String get variantLabel {
    final parts = <String>[];

    // Add frame effects first (showcase, borderless, etc.)
    if (frameEffects != null) {
      for (final effect in frameEffects!) {
        final label = _frameEffectLabel(effect);
        if (label != null) parts.add(label);
      }
    }

    // Add finish (foil, etched)
    if (finishes != null) {
      if (finishes!.contains('etched')) {
        parts.add('Etched Foil');
      } else if (finishes!.contains('foil') && !finishes!.contains('nonfoil')) {
        parts.add('Foil');
      } else if (finishes!.length == 1 && finishes!.contains('nonfoil')) {
        // Only nonfoil available
        if (parts.isEmpty) parts.add('Normal');
      }
    }

    if (parts.isEmpty) parts.add('Normal');
    return parts.join(' ');
  }

  String? _frameEffectLabel(String effect) {
    switch (effect) {
      case 'showcase':
        return 'Showcase';
      case 'extendedart':
        return 'Extended Art';
      case 'borderless':
        return 'Borderless';
      case 'retro_frame':
        return 'Retro Frame';
      case 'inverted':
        return 'Inverted';
      case 'fullart':
        return 'Full Art';
      default:
        return null;
    }
  }
}

class CardSearchResult {
  final List<CardModel> cards;
  final int totalCount;
  final bool hasMore;

  CardSearchResult({
    required this.cards,
    required this.totalCount,
    required this.hasMore,
  });

  factory CardSearchResult.fromJson(Map<String, dynamic> json) {
    return CardSearchResult(
      cards:
          (json['cards'] as List<dynamic>?)
              ?.map((c) => CardModel.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
      totalCount: json['total_count'] ?? 0,
      hasMore: json['has_more'] ?? false,
    );
  }
}

/// Metadata parsed from OCR scan
class ScanMetadata {
  final String? cardName;
  final String? cardNumber;
  final String? setTotal;
  final String? setCode;
  final String? setName;
  final String? hp;
  final String? rarity;
  final bool isFoil;
  final List<String> foilIndicators;
  final double confidence;
  final List<String> conditionHints;
  // Image analysis fields
  final String? suggestedCondition;
  final double? edgeWhiteningScore;
  final Map<String, double>? cornerScores;
  final double? foilConfidence;
  // First edition detection
  final bool isFirstEdition;
  final List<String> firstEdIndicators;
  // Set identification metadata
  final String?
  matchReason; // How set was determined: "set_code", "set_name", "unique_set_total", "inferred_from_total"
  final List<String> candidateSets; // Possible sets when ambiguous
  // Reverse holo detection
  final bool isReverseHolo;
  // Language detection
  final String? detectedLanguage; // e.g., "English", "Japanese", "German"

  ScanMetadata({
    this.cardName,
    this.cardNumber,
    this.setTotal,
    this.setCode,
    this.setName,
    this.hp,
    this.rarity,
    this.isFoil = false,
    this.foilIndicators = const [],
    this.confidence = 0.0,
    this.conditionHints = const [],
    this.suggestedCondition,
    this.edgeWhiteningScore,
    this.cornerScores,
    this.foilConfidence,
    this.isFirstEdition = false,
    this.firstEdIndicators = const [],
    this.matchReason,
    this.candidateSets = const [],
    this.isReverseHolo = false,
    this.detectedLanguage,
  });

  factory ScanMetadata.fromJson(Map<String, dynamic> json) {
    return ScanMetadata(
      cardName: json['card_name'],
      cardNumber: json['card_number'],
      setTotal: json['set_total'],
      setCode: json['set_code'],
      setName: json['set_name'],
      hp: json['hp'],
      rarity: json['rarity'],
      isFoil: json['is_foil'] ?? false,
      foilIndicators:
          (json['foil_indicators'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0.0,
      conditionHints:
          (json['condition_hints'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      suggestedCondition: json['suggested_condition'],
      edgeWhiteningScore: (json['edge_whitening_score'] as num?)?.toDouble(),
      cornerScores: (json['corner_scores'] as Map<String, dynamic>?)?.map(
        (key, value) => MapEntry(key, (value as num).toDouble()),
      ),
      foilConfidence: (json['foil_confidence'] as num?)?.toDouble(),
      isFirstEdition: json['is_first_edition'] ?? false,
      firstEdIndicators:
          (json['first_ed_indicators'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      matchReason: json['match_reason'],
      candidateSets:
          (json['candidate_sets'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      isReverseHolo: json['is_reverse_holo'] ?? false,
      detectedLanguage: json['detected_language'],
    );
  }

  /// Returns a human-readable summary of detected info
  String get detectionSummary {
    final parts = <String>[];
    if (cardName != null && cardName!.isNotEmpty) {
      parts.add('Name: $cardName');
    }
    if (setCode != null && setCode!.isNotEmpty) {
      parts.add('Set: ${setName ?? setCode}');
    }
    if (cardNumber != null && cardNumber!.isNotEmpty) {
      parts.add('#$cardNumber${setTotal != null ? "/$setTotal" : ""}');
    }
    if (rarity != null && rarity!.isNotEmpty) {
      parts.add(rarity!);
    }
    if (detectedLanguage != null && detectedLanguage != 'English') {
      parts.add(detectedLanguage!);
    }
    if (isFirstEdition) {
      parts.add('1st Edition');
    }
    if (isFoil) {
      final confPct = foilConfidence != null
          ? ' (${(foilConfidence! * 100).toInt()}%)'
          : '';
      parts.add('Foil detected$confPct');
    }
    if (suggestedCondition != null) {
      parts.add('Condition: $suggestedCondition');
    }
    return parts.isEmpty ? 'No details detected' : parts.join(' • ');
  }

  /// Returns true if the card is non-English (Japanese, German, etc.)
  bool get isNonEnglish =>
      detectedLanguage != null && detectedLanguage != 'English';

  /// Returns true if image analysis detected foil with high confidence
  bool get hasHighConfidenceFoil =>
      foilConfidence != null && foilConfidence! >= 0.7;

  /// Returns true if condition assessment suggests wear
  bool get hasConditionIssues =>
      suggestedCondition != null &&
      (suggestedCondition == 'MP' || suggestedCondition == 'HP');

  /// Returns true if the set identification is ambiguous (multiple candidate sets)
  bool get isSetAmbiguous => candidateSets.length > 1;

  /// Returns a human-readable description of how the set was matched
  String get matchReasonDescription {
    switch (matchReason) {
      case 'set_code':
        return 'Set code detected in text';
      case 'set_name':
        return 'Set name detected in text';
      case 'ptcgo_code':
        return 'PTCGO code detected';
      case 'unique_set_total':
        return 'Unique set size ($setTotal cards)';
      case 'inferred_from_total':
        return 'Inferred from set size (${candidateSets.length} possible sets)';
      default:
        return 'Name match only';
    }
  }

  /// Returns true if the set was identified with high confidence
  bool get hasHighConfidenceSet =>
      matchReason == 'set_code' ||
      matchReason == 'set_name' ||
      matchReason == 'ptcgo_code' ||
      matchReason == 'unique_set_total';

  /// Get suggested PrintingType based on OCR detection
  /// Priority: 1st Edition > Reverse Holo > Foil > Normal
  PrintingType get suggestedPrinting {
    if (isFirstEdition) return PrintingType.firstEdition;
    if (isReverseHolo) return PrintingType.reverseHolofoil;
    if (isFoil) return PrintingType.foil;
    return PrintingType.normal;
  }
}

/// Result from card identification (OCR scan)
class SetIconCandidate {
  final String setId;
  final double score;

  SetIconCandidate({required this.setId, required this.score});

  factory SetIconCandidate.fromJson(Map<String, dynamic> json) {
    return SetIconCandidate(
      setId: json['set_id'] ?? '',
      score: (json['score'] as num?)?.toDouble() ?? 0.0,
    );
  }
}

class SetIconResult {
  final String bestSetId;
  final double confidence;
  final bool lowConfidence;
  final List<SetIconCandidate> candidates;

  SetIconResult({
    required this.bestSetId,
    required this.confidence,
    required this.lowConfidence,
    required this.candidates,
  });

  factory SetIconResult.fromJson(Map<String, dynamic> json) {
    return SetIconResult(
      bestSetId: json['best_set_id'] ?? '',
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0.0,
      lowConfidence: json['low_confidence'] ?? false,
      candidates:
          (json['candidates'] as List<dynamic>?)
              ?.map((c) => SetIconCandidate.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

/// Result from card identification (OCR scan)
class ScanResult {
  final List<CardModel> cards;
  final int totalCount;
  final bool hasMore;
  final ScanMetadata metadata;
  final SetIconResult? setIcon;
  final MTGGroupedResult? grouped; // For MTG 2-phase selection
  final String? ocrText; // Original OCR text for caching Japanese translations

  ScanResult({
    required this.cards,
    required this.totalCount,
    required this.hasMore,
    required this.metadata,
    this.setIcon,
    this.grouped,
    this.ocrText,
  });

  factory ScanResult.fromJson(Map<String, dynamic> json) {
    final parsedData = json['parsed'];
    final setIconData = json['set_icon'];
    final groupedData = json['grouped'];

    return ScanResult(
      cards:
          (json['cards'] as List<dynamic>?)
              ?.map((c) => CardModel.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
      totalCount: json['total_count'] ?? 0,
      hasMore: json['has_more'] ?? false,
      metadata: ScanMetadata.fromJson(
        parsedData != null ? Map<String, dynamic>.from(parsedData) : {},
      ),
      setIcon: setIconData is Map<String, dynamic>
          ? SetIconResult.fromJson(setIconData)
          : null,
      grouped: groupedData is Map<String, dynamic>
          ? MTGGroupedResult.fromJson(groupedData)
          : null,
      ocrText:
          json['ocr_text'] as String?, // For Japanese card translation caching
    );
  }
}
