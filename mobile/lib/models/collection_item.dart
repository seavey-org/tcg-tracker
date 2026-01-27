import 'card.dart';

/// Printing type enum matching backend PrintingType
enum PrintingType {
  normal('Normal'),
  foil('Foil'),
  firstEdition('1st Edition'),
  unlimited('Unlimited'),
  reverseHolofoil('Reverse Holofoil');

  final String value;
  const PrintingType(this.value);

  static PrintingType fromString(String? value) {
    if (value == null || value.isEmpty) return PrintingType.normal;
    return PrintingType.values.firstWhere(
      (e) => e.value == value,
      orElse: () => PrintingType.normal,
    );
  }

  /// Returns true if this printing type should use foil pricing
  /// Matches backend logic in card_price.go PrintingType.IsFoilVariant()
  bool get usesFoilPricing =>
      this == PrintingType.foil ||
      this == PrintingType.firstEdition ||
      this == PrintingType.reverseHolofoil;
}

class CollectionItem {
  final int id;
  final String cardId;
  final CardModel card;
  final int quantity;
  final String condition;
  final PrintingType printing;
  final String? notes;
  final DateTime addedAt;
  final String? scannedImagePath;

  /// Backend-calculated value using condition-specific pricing
  final double? itemValue;

  /// Which language's price was used (may differ from card language if fallback)
  final String? priceLanguage;

  /// True if the price is from a different language than the card's language
  final bool priceFallback;

  CollectionItem({
    required this.id,
    required this.cardId,
    required this.card,
    required this.quantity,
    required this.condition,
    required this.printing,
    this.notes,
    required this.addedAt,
    this.scannedImagePath,
    this.itemValue,
    this.priceLanguage,
    this.priceFallback = false,
  });

  factory CollectionItem.fromJson(Map<String, dynamic> json) {
    return CollectionItem(
      id: json['id'] ?? 0,
      cardId: json['card_id'] ?? '',
      card: CardModel.fromJson(json['card'] ?? {}),
      quantity: json['quantity'] ?? 1,
      condition: json['condition'] ?? 'NM',
      printing: PrintingType.fromString(json['printing']),
      notes: json['notes'],
      addedAt: json['added_at'] != null
          ? DateTime.parse(json['added_at'])
          : DateTime.now(),
      scannedImagePath: json['scanned_image_path'],
      itemValue: (json['item_value'] as num?)?.toDouble(),
      priceLanguage: json['price_language'],
      priceFallback: json['price_fallback'] ?? false,
    );
  }

  /// Get total value of this item.
  /// Prefers backend-calculated itemValue (condition-specific) when available,
  /// otherwise falls back to card base price calculation.
  double get totalValue {
    if (itemValue != null) {
      return itemValue!;
    }
    // Fallback to base card price calculation
    final price = printing.usesFoilPricing
        ? (card.priceFoilUsd ?? card.priceUsd)
        : card.priceUsd;
    return (price ?? 0) * quantity;
  }

  /// Format the total value for display
  String get displayTotalValue {
    final value = totalValue;
    if (value == 0) return 'N/A';
    return '\$${value.toStringAsFixed(2)}';
  }

  /// Get display price (foil or regular based on this item's printing type)
  String get displayPrice {
    final price = printing.usesFoilPricing
        ? (card.priceFoilUsd ?? card.priceUsd)
        : card.priceUsd;
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
    PrintingType? printing,
    String? notes,
    DateTime? addedAt,
    String? scannedImagePath,
    double? itemValue,
    String? priceLanguage,
    bool? priceFallback,
  }) {
    return CollectionItem(
      id: id ?? this.id,
      cardId: cardId ?? this.cardId,
      card: card ?? this.card,
      quantity: quantity ?? this.quantity,
      condition: condition ?? this.condition,
      printing: printing ?? this.printing,
      notes: notes ?? this.notes,
      addedAt: addedAt ?? this.addedAt,
      scannedImagePath: scannedImagePath ?? this.scannedImagePath,
      itemValue: itemValue ?? this.itemValue,
      priceLanguage: priceLanguage ?? this.priceLanguage,
      priceFallback: priceFallback ?? this.priceFallback,
    );
  }
}
