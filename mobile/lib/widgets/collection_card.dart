import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../models/grouped_collection.dart';

/// A card widget that displays either a CollectionItem, GroupedCollectionItem, or a CardModel
/// Used in both collection grid and search results
class CollectionCard extends StatelessWidget {
  final CollectionItem? collectionItem;
  final GroupedCollectionItem? groupedItem;
  final CardModel? card;
  final VoidCallback? onTap;

  const CollectionCard({
    super.key,
    this.collectionItem,
    this.groupedItem,
    this.card,
    this.onTap,
  }) : assert(
         collectionItem != null || groupedItem != null || card != null,
         'Either collectionItem, groupedItem, or card must be provided',
       );

  CardModel get _card => collectionItem?.card ?? groupedItem?.card ?? card!;
  bool get _isCollectionItem => collectionItem != null;
  bool get _isGroupedItem => groupedItem != null;
  int? get _quantity => collectionItem?.quantity ?? groupedItem?.totalQuantity;
  bool get _isFoilVariant => collectionItem?.printing.usesFoilPricing ?? false;
  int get _scannedCount => groupedItem?.scannedCount ?? 0;
  bool get _hasMultipleVariants =>
      groupedItem != null && groupedItem!.variants.length > 1;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Card(
      clipBehavior: Clip.antiAlias,
      elevation: 2,
      child: InkWell(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // Card image with badges
            Expanded(
              flex: 3,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  // Card image
                  _buildImage(colorScheme),
                  // Quantity badge (for collection or grouped items)
                  if ((_isCollectionItem || _isGroupedItem) &&
                      _quantity != null &&
                      _quantity! > 1)
                    Positioned(
                      top: 4,
                      right: 4,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: colorScheme.primary,
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Text(
                          'x$_quantity',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: colorScheme.onPrimary,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ),
                  // Foil indicator (shows for any foil-priced variant)
                  // For grouped items, show "+" if multiple variants
                  if (_isFoilVariant || _hasMultipleVariants)
                    Positioned(
                      top: 4,
                      left: 4,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          gradient: LinearGradient(
                            colors: [
                              Colors.purple.shade300,
                              Colors.blue.shade300,
                            ],
                          ),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Text(
                          _hasMultipleVariants ? 'FOIL+' : 'FOIL',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: Colors.white,
                            fontWeight: FontWeight.bold,
                            fontSize: 10,
                          ),
                        ),
                      ),
                    ),
                  // Scanned count indicator (for grouped items)
                  if (_isGroupedItem && _scannedCount > 0)
                    Positioned(
                      bottom: 4,
                      right: 4,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: Colors.green.shade700,
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(
                              Icons.camera_alt,
                              size: 10,
                              color: Colors.white,
                            ),
                            const SizedBox(width: 2),
                            Text(
                              '$_scannedCount',
                              style: theme.textTheme.labelSmall?.copyWith(
                                color: Colors.white,
                                fontWeight: FontWeight.bold,
                                fontSize: 10,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  // Game badge
                  Positioned(
                    bottom: 4,
                    left: 4,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: _card.game == 'mtg'
                            ? Colors.purple.shade700
                            : Colors.amber.shade700,
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Text(
                        _card.game.toUpperCase(),
                        style: theme.textTheme.labelSmall?.copyWith(
                          color: Colors.white,
                          fontWeight: FontWeight.bold,
                          fontSize: 9,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            // Card info
            Expanded(
              flex: 2,
              child: Padding(
                padding: const EdgeInsets.all(8.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Card name
                    Text(
                      _card.name,
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const Spacer(),
                    // Set name
                    Text(
                      _card.displaySet,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: colorScheme.onSurfaceVariant,
                        fontSize: 10,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 2),
                    // Per-card price or total value
                    Text(
                      _isGroupedItem
                          ? groupedItem!.displayTotalValue
                          : _isCollectionItem
                          ? collectionItem!.displayPrice
                          : _card.displayPrice,
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: colorScheme.primary,
                      ),
                    ),
                    // Total value label for grouped items or collection items with qty > 1
                    if (_isGroupedItem && _quantity != null && _quantity! > 1)
                      Text(
                        'Total (x$_quantity)',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontSize: 9,
                          color: colorScheme.onSurfaceVariant,
                        ),
                      )
                    else if (_isCollectionItem &&
                        _quantity != null &&
                        _quantity! > 1)
                      Text(
                        'Total: ${collectionItem!.displayTotalValue}',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontSize: 9,
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImage(ColorScheme colorScheme) {
    if (_card.imageUrl == null || _card.imageUrl!.isEmpty) {
      return Container(
        color: colorScheme.surfaceContainerHighest,
        child: Icon(
          Icons.image_not_supported_outlined,
          size: 48,
          color: colorScheme.onSurfaceVariant,
        ),
      );
    }

    return CachedNetworkImage(
      imageUrl: _card.imageUrl!,
      fit: BoxFit.cover,
      placeholder: (context, url) => Container(
        color: colorScheme.surfaceContainerHighest,
        child: const Center(child: CircularProgressIndicator(strokeWidth: 2)),
      ),
      errorWidget: (context, url, error) => Container(
        color: colorScheme.surfaceContainerHighest,
        child: Icon(
          Icons.broken_image_outlined,
          size: 48,
          color: colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}
