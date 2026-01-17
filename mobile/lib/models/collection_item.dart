import 'card.dart';

class CollectionItem {
  final int id;
  final String cardId;
  final CardModel card;
  final int quantity;
  final String condition;
  final bool foil;
  final String? notes;
  final DateTime addedAt;

  CollectionItem({
    required this.id,
    required this.cardId,
    required this.card,
    required this.quantity,
    required this.condition,
    required this.foil,
    this.notes,
    required this.addedAt,
  });

  factory CollectionItem.fromJson(Map<String, dynamic> json) {
    return CollectionItem(
      id: json['id'] ?? 0,
      cardId: json['card_id'] ?? '',
      card: CardModel.fromJson(json['card'] ?? {}),
      quantity: json['quantity'] ?? 1,
      condition: json['condition'] ?? 'NM',
      foil: json['foil'] ?? false,
      notes: json['notes'],
      addedAt: json['added_at'] != null
          ? DateTime.parse(json['added_at'])
          : DateTime.now(),
    );
  }

  /// Calculate total value of this item (quantity * appropriate price)
  double get totalValue {
    final price = foil ? (card.priceFoilUsd ?? card.priceUsd) : card.priceUsd;
    return (price ?? 0) * quantity;
  }

  /// Format the total value for display
  String get displayTotalValue {
    final value = totalValue;
    if (value == 0) return 'N/A';
    return '\$${value.toStringAsFixed(2)}';
  }

  /// Get display price (foil or regular based on this item's foil status)
  String get displayPrice {
    final price = foil ? (card.priceFoilUsd ?? card.priceUsd) : card.priceUsd;
    if (price == null || price == 0) return 'N/A';
    return '\$${price.toStringAsFixed(2)}';
  }

  /// Create a copy with updated fields
  CollectionItem copyWith({
    int? id,
    String? cardId,
    CardModel? card,
    int? quantity,
    String? condition,
    bool? foil,
    String? notes,
    DateTime? addedAt,
  }) {
    return CollectionItem(
      id: id ?? this.id,
      cardId: cardId ?? this.cardId,
      card: card ?? this.card,
      quantity: quantity ?? this.quantity,
      condition: condition ?? this.condition,
      foil: foil ?? this.foil,
      notes: notes ?? this.notes,
      addedAt: addedAt ?? this.addedAt,
    );
  }
}
