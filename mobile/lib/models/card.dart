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
