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
      cards: (json['cards'] as List<dynamic>?)
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
      foilIndicators: (json['foil_indicators'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0.0,
      conditionHints: (json['condition_hints'] as List<dynamic>?)
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
    if (isFoil) {
      parts.add('Foil detected');
    }
    return parts.isEmpty ? 'No details detected' : parts.join(' • ');
  }
}

/// Result from card identification (OCR scan)
class ScanResult {
  final List<CardModel> cards;
  final int totalCount;
  final bool hasMore;
  final ScanMetadata metadata;

  ScanResult({
    required this.cards,
    required this.totalCount,
    required this.hasMore,
    required this.metadata,
  });

  factory ScanResult.fromJson(Map<String, dynamic> json) {
    final parsedData = json['parsed'];
    return ScanResult(
      cards: (json['cards'] as List<dynamic>?)
              ?.map((c) => CardModel.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
      totalCount: json['total_count'] ?? 0,
      hasMore: json['has_more'] ?? false,
      metadata: ScanMetadata.fromJson(
        parsedData != null ? Map<String, dynamic>.from(parsedData) : {},
      ),
    );
  }
}
