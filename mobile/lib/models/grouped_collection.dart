import 'card.dart';
import 'collection_item.dart';

/// Represents a collection variant (same card with specific printing+condition)
class CollectionVariant {
  final PrintingType printing;
  final String condition;
  final int quantity;
  final double value;
  final bool hasScans;
  final int scannedQty;

  CollectionVariant({
    required this.printing,
    required this.condition,
    required this.quantity,
    required this.value,
    required this.hasScans,
    required this.scannedQty,
  });

  factory CollectionVariant.fromJson(Map<String, dynamic> json) {
    return CollectionVariant(
      printing: PrintingType.fromString(json['printing']),
      condition: json['condition'] ?? 'NM',
      quantity: json['quantity'] ?? 0,
      value: (json['value'] ?? 0).toDouble(),
      hasScans: json['has_scans'] ?? false,
      scannedQty: json['scanned_qty'] ?? 0,
    );
  }
}

/// Represents a card with all its collection entries grouped
class GroupedCollectionItem {
  final CardModel card;
  final int totalQuantity;
  final double totalValue;
  final int scannedCount;
  final List<CollectionVariant> variants;
  final List<CollectionItem> items;

  GroupedCollectionItem({
    required this.card,
    required this.totalQuantity,
    required this.totalValue,
    required this.scannedCount,
    required this.variants,
    required this.items,
  });

  factory GroupedCollectionItem.fromJson(Map<String, dynamic> json) {
    return GroupedCollectionItem(
      card: CardModel.fromJson(json['card'] ?? {}),
      totalQuantity: json['total_quantity'] ?? 0,
      totalValue: (json['total_value'] ?? 0).toDouble(),
      scannedCount: json['scanned_count'] ?? 0,
      variants:
          (json['variants'] as List<dynamic>?)
              ?.map(
                (v) => CollectionVariant.fromJson(v as Map<String, dynamic>),
              )
              .toList() ??
          [],
      items:
          (json['items'] as List<dynamic>?)
              ?.map((i) => CollectionItem.fromJson(i as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  /// Get all scanned items
  List<CollectionItem> get scannedItems =>
      items.where((i) => i.scannedImagePath != null).toList();

  /// Format the total value for display
  String get displayTotalValue {
    if (totalValue == 0) return 'N/A';
    return '\$${totalValue.toStringAsFixed(2)}';
  }
}

/// Response from update operation including operation info
class CollectionUpdateResponse {
  final CollectionItem item;
  final String
  operation; // 'updated', 'split', 'merged', 'reassigned', 'reassigned_merged'
  final String? message;

  CollectionUpdateResponse({
    required this.item,
    required this.operation,
    this.message,
  });

  factory CollectionUpdateResponse.fromJson(Map<String, dynamic> json) {
    return CollectionUpdateResponse(
      item: CollectionItem.fromJson(json['item'] ?? {}),
      operation: json['operation'] ?? 'updated',
      message: json['message'],
    );
  }

  bool get isSplit => operation == 'split';
  bool get isMerged => operation == 'merged';
  bool get isReassigned =>
      operation == 'reassigned' || operation == 'reassigned_merged';
}
