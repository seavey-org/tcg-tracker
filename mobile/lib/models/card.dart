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
    );
  }

  String get displayPrice {
    if (priceUsd == null || priceUsd == 0) return 'N/A';
    return '\$${priceUsd!.toStringAsFixed(2)}';
  }

  String get displaySet => setName ?? setCode ?? 'Unknown Set';
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

  /// Returns true if image analysis detected foil with high confidence
  bool get hasHighConfidenceFoil =>
      foilConfidence != null && foilConfidence! >= 0.7;

  /// Returns true if condition assessment suggests wear
  bool get hasConditionIssues =>
      suggestedCondition != null &&
      (suggestedCondition == 'MP' || suggestedCondition == 'HP');
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

  ScanResult({
    required this.cards,
    required this.totalCount,
    required this.hasMore,
    required this.metadata,
    this.setIcon,
  });

  factory ScanResult.fromJson(Map<String, dynamic> json) {
    final parsedData = json['parsed'];
    final setIconData = json['set_icon'];

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
    );
  }
}
