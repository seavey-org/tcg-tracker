import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../models/grouped_collection.dart';
import '../providers/collection_provider.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../utils/constants.dart';
import '../widgets/admin_key_dialog.dart';

class CardDetailScreen extends StatefulWidget {
  final CollectionItem? collectionItem;
  final GroupedCollectionItem? groupedItem;
  final CardModel? card;

  const CardDetailScreen({
    super.key,
    this.collectionItem,
    this.groupedItem,
    this.card,
  }) : assert(
         collectionItem != null || groupedItem != null || card != null,
         'Either collectionItem, groupedItem, or card must be provided',
       );

  @override
  State<CardDetailScreen> createState() => _CardDetailScreenState();
}

class _CardDetailScreenState extends State<CardDetailScreen>
    with SingleTickerProviderStateMixin {
  late int _quantity;
  late String _condition;
  late PrintingType _printing;
  bool _loading = false;
  bool _priceRefreshing = false;
  bool _showScannedImage = false;

  // For grouped item editing
  TabController? _tabController;

  CardModel get _card =>
      widget.collectionItem?.card ?? widget.groupedItem?.card ?? widget.card!;
  bool get _isCollectionItem => widget.collectionItem != null;
  bool get _isGroupedItem => widget.groupedItem != null;
  bool get _hasScannedImage => widget.collectionItem?.scannedImagePath != null;

  // Use unified condition codes from constants
  List<String> get _conditions => CardConditions.codes;
  Map<String, String> get _conditionLabels => CardConditions.labels;

  // Helper to get a short badge label for printing type
  String _printingBadgeLabel(PrintingType printing) {
    switch (printing) {
      case PrintingType.foil:
        return 'FOIL';
      case PrintingType.firstEdition:
        return '1ST ED';
      case PrintingType.reverseHolofoil:
        return 'REV HOLO';
      case PrintingType.unlimited:
        return 'UNLTD';
      case PrintingType.normal:
        return '';
    }
  }

  // Helper to get display name for printing type dropdown
  String _printingDisplayName(PrintingType printing) {
    switch (printing) {
      case PrintingType.normal:
        return 'Normal';
      case PrintingType.foil:
        return 'Foil / Holo';
      case PrintingType.firstEdition:
        return '1st Edition';
      case PrintingType.reverseHolofoil:
        return 'Reverse Holo';
      case PrintingType.unlimited:
        return 'Unlimited';
    }
  }

  @override
  void initState() {
    super.initState();
    _quantity = widget.collectionItem?.quantity ?? 1;
    _condition = widget.collectionItem?.condition ?? 'NM';
    _printing = widget.collectionItem?.printing ?? PrintingType.normal;

    // Initialize tab controller for grouped items
    if (_isGroupedItem) {
      _tabController = TabController(length: 3, vsync: this);
    }
  }

  @override
  void dispose() {
    _tabController?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final mediaQuery = MediaQuery.of(context);
    final isWide = mediaQuery.size.width > 600;

    // For grouped items, use a different layout with tabs
    if (_isGroupedItem) {
      return _buildGroupedItemScreen(context, isWide);
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(_card.name),
        actions: [
          if (_isCollectionItem)
            IconButton(
              icon: const Icon(Icons.delete_outline),
              onPressed: _confirmDelete,
              tooltip: 'Delete',
            ),
        ],
      ),
      body: SafeArea(
        child: isWide ? _buildWideLayout(context) : _buildNarrowLayout(context),
      ),
    );
  }

  Widget _buildGroupedItemScreen(BuildContext context, bool isWide) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final grouped = widget.groupedItem!;

    return Scaffold(
      appBar: AppBar(
        title: Text(_card.name),
        bottom: TabBar(
          controller: _tabController,
          tabs: [
            Tab(
              icon: const Icon(Icons.layers),
              text: 'Variants (${grouped.variants.length})',
            ),
            Tab(
              icon: const Icon(Icons.camera_alt),
              text: 'Scans (${grouped.scannedCount})',
            ),
            Tab(
              icon: const Icon(Icons.list),
              text: 'Items (${grouped.items.length})',
            ),
          ],
        ),
      ),
      body: SafeArea(
        child: Column(
          children: [
            // Card image and summary at top
            Container(
              padding: const EdgeInsets.all(16),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Card thumbnail
                  SizedBox(
                    width: 100,
                    child: AspectRatio(
                      aspectRatio: 2.5 / 3.5,
                      child: Card(
                        clipBehavior: Clip.antiAlias,
                        child: _card.imageUrl != null
                            ? CachedNetworkImage(
                                imageUrl: _card.imageUrl!,
                                fit: BoxFit.cover,
                              )
                            : Container(
                                color: colorScheme.surfaceContainerHighest,
                                child: Icon(
                                  Icons.image_not_supported,
                                  color: colorScheme.onSurfaceVariant,
                                ),
                              ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  // Summary info
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          _card.displaySet,
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: 8),
                        Row(
                          children: [
                            Text(
                              'x${grouped.totalQuantity}',
                              style: theme.textTheme.headlineSmall?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                            const SizedBox(width: 16),
                            Text(
                              grouped.displayTotalValue,
                              style: theme.textTheme.headlineSmall?.copyWith(
                                fontWeight: FontWeight.bold,
                                color: colorScheme.primary,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text(
                          '${grouped.variants.length} variant${grouped.variants.length > 1 ? 's' : ''}, '
                          '${grouped.scannedCount} scanned',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const Divider(height: 1),
            // Tab content
            Expanded(
              child: TabBarView(
                controller: _tabController,
                children: [
                  _buildVariantsTab(context, grouped),
                  _buildScansTab(context, grouped),
                  _buildItemsTab(context, grouped),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildVariantsTab(
    BuildContext context,
    GroupedCollectionItem grouped,
  ) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    if (grouped.variants.isEmpty) {
      return Center(
        child: Text(
          'No variants',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: colorScheme.onSurfaceVariant,
          ),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: grouped.variants.length,
      itemBuilder: (context, index) {
        final variant = grouped.variants[index];
        return Card(
          margin: const EdgeInsets.only(bottom: 12),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                // Printing badge
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    gradient: variant.printing.usesFoilPricing
                        ? LinearGradient(
                            colors: [
                              Colors.purple.shade300,
                              Colors.blue.shade300,
                            ],
                          )
                        : null,
                    color: variant.printing.usesFoilPricing
                        ? null
                        : colorScheme.surfaceContainerHighest,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Text(
                    _printingDisplayName(variant.printing),
                    style: theme.textTheme.labelMedium?.copyWith(
                      color: variant.printing.usesFoilPricing
                          ? Colors.white
                          : colorScheme.onSurface,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                // Condition
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: colorScheme.secondaryContainer,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Text(
                    variant.condition,
                    style: theme.textTheme.labelMedium?.copyWith(
                      color: colorScheme.onSecondaryContainer,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                const Spacer(),
                // Quantity and value
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Text(
                      'x${variant.quantity}',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    Text(
                      '\$${variant.value.toStringAsFixed(2)}',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: colorScheme.primary,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildScansTab(BuildContext context, GroupedCollectionItem grouped) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final scannedItems = grouped.scannedItems;

    if (scannedItems.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.camera_alt_outlined,
              size: 64,
              color: colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
            ),
            const SizedBox(height: 16),
            Text(
              'No scanned cards',
              style: theme.textTheme.titleMedium?.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Scanned cards will appear here for\ncondition verification',
              style: theme.textTheme.bodySmall?.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      );
    }

    return GridView.builder(
      padding: const EdgeInsets.all(16),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 2,
        mainAxisSpacing: 12,
        crossAxisSpacing: 12,
        childAspectRatio: 2.5 / 3.5,
      ),
      itemCount: scannedItems.length,
      itemBuilder: (context, index) {
        final item = scannedItems[index];
        return GestureDetector(
          onTap: () => _editItem(item),
          child: Card(
            clipBehavior: Clip.antiAlias,
            child: Stack(
              fit: StackFit.expand,
              children: [
                FutureBuilder<String>(
                  future: ApiService().getServerUrl(),
                  builder: (context, snapshot) {
                    if (!snapshot.hasData) {
                      return Container(
                        color: colorScheme.surfaceContainerHighest,
                        child: const Center(child: CircularProgressIndicator()),
                      );
                    }
                    final imageUrl =
                        '${snapshot.data}/images/scanned/${item.scannedImagePath}';
                    return CachedNetworkImage(
                      imageUrl: imageUrl,
                      fit: BoxFit.cover,
                      placeholder: (context, url) => Container(
                        color: colorScheme.surfaceContainerHighest,
                        child: const Center(child: CircularProgressIndicator()),
                      ),
                      errorWidget: (context, url, error) => Container(
                        color: colorScheme.surfaceContainerHighest,
                        child: Icon(
                          Icons.broken_image,
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                    );
                  },
                ),
                // Overlay with condition/printing info
                Positioned(
                  bottom: 0,
                  left: 0,
                  right: 0,
                  child: Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.bottomCenter,
                        end: Alignment.topCenter,
                        colors: [
                          Colors.black.withValues(alpha: 0.8),
                          Colors.transparent,
                        ],
                      ),
                    ),
                    child: Row(
                      children: [
                        Text(
                          item.condition,
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: Colors.white,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const Spacer(),
                        if (item.printing != PrintingType.normal)
                          Text(
                            _printingBadgeLabel(item.printing),
                            style: theme.textTheme.labelSmall?.copyWith(
                              color: Colors.white,
                              fontWeight: FontWeight.bold,
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
      },
    );
  }

  Widget _buildItemsTab(BuildContext context, GroupedCollectionItem grouped) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    if (grouped.items.isEmpty) {
      return Center(
        child: Text(
          'No items',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: colorScheme.onSurfaceVariant,
          ),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: grouped.items.length,
      itemBuilder: (context, index) {
        final item = grouped.items[index];
        final isScanned = item.scannedImagePath != null;

        return Card(
          margin: const EdgeInsets.only(bottom: 12),
          child: InkWell(
            onTap: () => _editItem(item),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  // Scanned indicator
                  if (isScanned)
                    Container(
                      padding: const EdgeInsets.all(4),
                      decoration: BoxDecoration(
                        color: Colors.green.shade100,
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Icon(
                        Icons.camera_alt,
                        size: 16,
                        color: Colors.green.shade700,
                      ),
                    )
                  else
                    Container(
                      padding: const EdgeInsets.all(4),
                      decoration: BoxDecoration(
                        color: colorScheme.surfaceContainerHighest,
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Icon(
                        Icons.inventory_2,
                        size: 16,
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                  const SizedBox(width: 12),
                  // Item details
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Text(
                              _printingDisplayName(item.printing),
                              style: theme.textTheme.bodyMedium?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                            const SizedBox(width: 8),
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 6,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: colorScheme.secondaryContainer,
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: Text(
                                item.condition,
                                style: theme.textTheme.labelSmall?.copyWith(
                                  color: colorScheme.onSecondaryContainer,
                                ),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'Qty: ${item.quantity}${isScanned ? ' (scanned)' : ''}',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                  // Price and edit
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        item.displayTotalValue,
                        style: theme.textTheme.titleSmall?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: colorScheme.primary,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Icon(
                        Icons.edit,
                        size: 16,
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  void _editItem(CollectionItem item) {
    // Navigate to edit this specific item
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => CardDetailScreen(collectionItem: item),
      ),
    ).then((_) {
      // Refresh the grouped collection when we return
      if (mounted) {
        context.read<CollectionProvider>().fetchGroupedCollection();
      }
    });
  }

  Widget _buildWideLayout(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Image side
        Expanded(
          flex: 2,
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: _buildCardImage(context),
          ),
        ),
        // Details side
        Expanded(
          flex: 3,
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: _buildDetailsColumn(context),
          ),
        ),
      ],
    );
  }

  Widget _buildNarrowLayout(BuildContext context) {
    return SingleChildScrollView(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Card image
          Padding(
            padding: const EdgeInsets.all(16),
            child: _buildCardImage(context),
          ),
          // Details
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            child: _buildDetailsColumn(context),
          ),
        ],
      ),
    );
  }

  Widget _buildCardImage(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Column(
      children: [
        // Image toggle if scanned image exists
        if (_hasScannedImage) ...[
          SegmentedButton<bool>(
            segments: const [
              ButtonSegment(
                value: false,
                label: Text('Official'),
                icon: Icon(Icons.image_outlined),
              ),
              ButtonSegment(
                value: true,
                label: Text('My Scan'),
                icon: Icon(Icons.camera_alt_outlined),
              ),
            ],
            selected: {_showScannedImage},
            onSelectionChanged: (selection) {
              setState(() => _showScannedImage = selection.first);
            },
          ),
          const SizedBox(height: 12),
        ],
        // Card image
        AspectRatio(
          aspectRatio: 2.5 / 3.5,
          child: Card(
            clipBehavior: Clip.antiAlias,
            elevation: 4,
            child: _showScannedImage && _hasScannedImage
                ? FutureBuilder<String>(
                    future: ApiService().getServerUrl(),
                    builder: (context, snapshot) {
                      if (!snapshot.hasData) {
                        return Container(
                          color: colorScheme.surfaceContainerHighest,
                          child: const Center(
                            child: CircularProgressIndicator(),
                          ),
                        );
                      }
                      final imageUrl =
                          '${snapshot.data}/images/scanned/${widget.collectionItem!.scannedImagePath}';
                      return CachedNetworkImage(
                        imageUrl: imageUrl,
                        fit: BoxFit.cover,
                        placeholder: (context, url) => Container(
                          color: colorScheme.surfaceContainerHighest,
                          child: const Center(
                            child: CircularProgressIndicator(),
                          ),
                        ),
                        errorWidget: (context, url, error) => Container(
                          color: colorScheme.surfaceContainerHighest,
                          child: Icon(
                            Icons.broken_image_outlined,
                            size: 64,
                            color: colorScheme.onSurfaceVariant,
                          ),
                        ),
                      );
                    },
                  )
                : _card.imageUrl != null && _card.imageUrl!.isNotEmpty
                ? CachedNetworkImage(
                    imageUrl: _card.imageUrl!,
                    fit: BoxFit.cover,
                    placeholder: (context, url) => Container(
                      color: colorScheme.surfaceContainerHighest,
                      child: const Center(child: CircularProgressIndicator()),
                    ),
                    errorWidget: (context, url, error) => Container(
                      color: colorScheme.surfaceContainerHighest,
                      child: Icon(
                        Icons.broken_image_outlined,
                        size: 64,
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                  )
                : Container(
                    color: colorScheme.surfaceContainerHighest,
                    child: Icon(
                      Icons.image_not_supported_outlined,
                      size: 64,
                      color: colorScheme.onSurfaceVariant,
                    ),
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildDetailsColumn(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Card info section
        _buildInfoSection(context),
        const SizedBox(height: 16),
        const Divider(),
        const SizedBox(height: 16),

        // Price section
        _buildPriceSection(context),
        const SizedBox(height: 16),
        const Divider(),
        const SizedBox(height: 16),

        // Edit section (for collection items) or Add section (for search results)
        if (_isCollectionItem)
          _buildEditSection(context)
        else
          _buildAddSection(context),
      ],
    );
  }

  Widget _buildInfoSection(BuildContext context) {
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Game badge and name
        Row(
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
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
                ),
              ),
            ),
            if (_printing != PrintingType.normal) ...[
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  gradient: _printing.usesFoilPricing
                      ? LinearGradient(
                          colors: [
                            Colors.purple.shade300,
                            Colors.blue.shade300,
                          ],
                        )
                      : null,
                  color: _printing.usesFoilPricing
                      ? null
                      : Colors.amber.shade700,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  _printingBadgeLabel(_printing),
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
            ],
          ],
        ),
        const SizedBox(height: 12),

        // Card details
        _buildDetailRow(context, 'Set', _card.displaySet),
        if (_card.setCode != null)
          _buildDetailRow(context, 'Set Code', _card.setCode!),
        if (_card.cardNumber != null)
          _buildDetailRow(context, 'Card Number', _card.cardNumber!),
        if (_card.rarity != null)
          _buildDetailRow(context, 'Rarity', _card.rarity!),
      ],
    );
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 100,
            child: Text(
              label,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(child: Text(value, style: theme.textTheme.bodyMedium)),
        ],
      ),
    );
  }

  Widget _buildPriceSection(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Prices',
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const Spacer(),
            if (_card.game == 'pokemon')
              TextButton.icon(
                onPressed: _priceRefreshing ? null : _refreshPrice,
                icon: _priceRefreshing
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.refresh, size: 18),
                label: const Text('Refresh'),
              ),
          ],
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Regular',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        _card.displayPrice,
                        style: theme.textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: colorScheme.primary,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Foil',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        _card.priceFoilUsd != null && _card.priceFoilUsd! > 0
                            ? '\$${_card.priceFoilUsd!.toStringAsFixed(2)}'
                            : 'N/A',
                        style: theme.textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: colorScheme.primary,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
        // TCGPlayer link
        if (_card.tcgplayerUrl != null) ...[
          const SizedBox(height: 12),
          OutlinedButton.icon(
            onPressed: () async {
              final url = Uri.parse(_card.tcgplayerUrl!);
              if (await canLaunchUrl(url)) {
                await launchUrl(url, mode: LaunchMode.externalApplication);
              }
            },
            icon: const Icon(Icons.open_in_new, size: 18),
            label: const Text('View on TCGPlayer'),
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(double.infinity, 44),
            ),
          ),
        ],
        // Total value for collection items with quantity > 1
        if (_isCollectionItem && _quantity > 1) ...[
          const SizedBox(height: 12),
          Card(
            color: colorScheme.primaryContainer,
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  Text(
                    'Total Value (x$_quantity)',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: colorScheme.onPrimaryContainer,
                    ),
                  ),
                  const Spacer(),
                  Text(
                    widget.collectionItem!.displayTotalValue,
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: colorScheme.onPrimaryContainer,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildEditSection(BuildContext context) {
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Collection Details',
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 16),

        // Quantity
        Row(
          children: [
            const Text('Quantity'),
            const Spacer(),
            IconButton(
              onPressed: _quantity > 1
                  ? () => setState(() => _quantity--)
                  : null,
              icon: const Icon(Icons.remove_circle_outline),
            ),
            Text(
              '$_quantity',
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            IconButton(
              onPressed: () => setState(() => _quantity++),
              icon: const Icon(Icons.add_circle_outline),
            ),
          ],
        ),
        const SizedBox(height: 12),

        // Condition
        DropdownButtonFormField<String>(
          initialValue: _condition,
          decoration: InputDecoration(
            labelText: 'Condition',
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
          ),
          items: _conditions.map((c) {
            return DropdownMenuItem(
              value: c,
              child: Text('$c (${_conditionLabels[c]})'),
            );
          }).toList(),
          onChanged: (value) {
            if (value != null) setState(() => _condition = value);
          },
        ),
        const SizedBox(height: 16),

        // Printing type
        DropdownButtonFormField<PrintingType>(
          initialValue: _printing,
          decoration: InputDecoration(
            labelText: 'Printing',
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
          ),
          items: PrintingType.values.map((p) {
            return DropdownMenuItem(
              value: p,
              child: Text(_printingDisplayName(p)),
            );
          }).toList(),
          onChanged: (value) {
            if (value != null) setState(() => _printing = value);
          },
        ),
        const SizedBox(height: 24),

        // Update button
        FilledButton(
          onPressed: _loading ? null : _updateItem,
          child: _loading
              ? const SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Text('Update'),
        ),
      ],
    );
  }

  Widget _buildAddSection(BuildContext context) {
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Add to Collection',
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 16),

        // Quantity
        Row(
          children: [
            const Text('Quantity'),
            const Spacer(),
            IconButton(
              onPressed: _quantity > 1
                  ? () => setState(() => _quantity--)
                  : null,
              icon: const Icon(Icons.remove_circle_outline),
            ),
            Text(
              '$_quantity',
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            IconButton(
              onPressed: () => setState(() => _quantity++),
              icon: const Icon(Icons.add_circle_outline),
            ),
          ],
        ),
        const SizedBox(height: 12),

        // Condition
        DropdownButtonFormField<String>(
          initialValue: _condition,
          decoration: InputDecoration(
            labelText: 'Condition',
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
          ),
          items: _conditions.map((c) {
            return DropdownMenuItem(
              value: c,
              child: Text('$c (${_conditionLabels[c]})'),
            );
          }).toList(),
          onChanged: (value) {
            if (value != null) setState(() => _condition = value);
          },
        ),
        const SizedBox(height: 16),

        // Printing type
        DropdownButtonFormField<PrintingType>(
          initialValue: _printing,
          decoration: InputDecoration(
            labelText: 'Printing',
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
          ),
          items: PrintingType.values.map((p) {
            return DropdownMenuItem(
              value: p,
              child: Text(_printingDisplayName(p)),
            );
          }).toList(),
          onChanged: (value) {
            if (value != null) setState(() => _printing = value);
          },
        ),
        const SizedBox(height: 24),

        // Add button
        FilledButton(
          onPressed: _loading ? null : _addToCollection,
          child: _loading
              ? const SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Text('Add to Collection'),
        ),
      ],
    );
  }

  Future<void> _updateItem() async {
    final provider = context.read<CollectionProvider>();
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    setState(() => _loading = true);

    try {
      final response = await provider.updateItem(
        widget.collectionItem!.id,
        quantity: _quantity,
        condition: _condition,
        printing: _printing,
      );
      if (!mounted) return;

      // Show appropriate message based on operation type
      String message;
      if (response.isSplit) {
        message = response.message ?? 'Card split into separate entries';
      } else if (response.isMerged) {
        message = response.message ?? 'Card merged with existing entry';
      } else {
        message = 'Card updated';
      }

      messenger.showSnackBar(
        SnackBar(content: Text(message), behavior: SnackBarBehavior.floating),
      );
      navigator.pop();
    } on AuthRequiredException {
      if (!mounted) return;
      setState(() => _loading = false);
      // Show admin key dialog and retry on success
      final success = await AdminKeyDialog.show(context, ApiService());
      if (success && mounted) {
        _updateItem(); // Retry after authentication
      }
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(
        SnackBar(
          content: Text('Error: $e'),
          backgroundColor: Theme.of(context).colorScheme.error,
          behavior: SnackBarBehavior.floating,
        ),
      );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _addToCollection() async {
    final provider = context.read<CollectionProvider>();
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    setState(() => _loading = true);

    try {
      await provider.addToCollection(
        _card.id,
        quantity: _quantity,
        condition: _condition,
        printing: _printing,
      );
      if (!mounted) return;
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Added to collection'),
          behavior: SnackBarBehavior.floating,
        ),
      );
      navigator.pop();
    } on AuthRequiredException {
      if (!mounted) return;
      setState(() => _loading = false);
      // Show admin key dialog and retry on success
      final success = await AdminKeyDialog.show(context, ApiService());
      if (success && mounted) {
        _addToCollection(); // Retry after authentication
      }
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(
        SnackBar(
          content: Text('Error: $e'),
          backgroundColor: Theme.of(context).colorScheme.error,
          behavior: SnackBarBehavior.floating,
        ),
      );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _refreshPrice() async {
    final provider = context.read<CollectionProvider>();
    final messenger = ScaffoldMessenger.of(context);

    setState(() => _priceRefreshing = true);

    try {
      await provider.refreshCardPrice(_card.id);
      if (!mounted) return;
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Price refreshed'),
          behavior: SnackBarBehavior.floating,
        ),
      );
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(
        SnackBar(
          content: Text('Error: $e'),
          backgroundColor: Theme.of(context).colorScheme.error,
          behavior: SnackBarBehavior.floating,
        ),
      );
    } finally {
      if (mounted) setState(() => _priceRefreshing = false);
    }
  }

  void _confirmDelete() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Card'),
        content: Text('Remove "${_card.name}" from your collection?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.pop(context);
              _deleteItem();
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
  }

  Future<void> _deleteItem() async {
    final provider = context.read<CollectionProvider>();
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    try {
      await provider.removeItem(widget.collectionItem!.id);
      if (!mounted) return;
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Card removed from collection'),
          behavior: SnackBarBehavior.floating,
        ),
      );
      navigator.pop();
    } on AuthRequiredException {
      if (!mounted) return;
      // Show admin key dialog and retry on success
      final success = await AdminKeyDialog.show(context, ApiService());
      if (success && mounted) {
        _deleteItem(); // Retry after authentication
      }
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(
        SnackBar(
          content: Text('Error: $e'),
          backgroundColor: Theme.of(context).colorScheme.error,
          behavior: SnackBarBehavior.floating,
        ),
      );
    }
  }
}
