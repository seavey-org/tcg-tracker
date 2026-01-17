import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:provider/provider.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../providers/collection_provider.dart';

class CardDetailScreen extends StatefulWidget {
  final CollectionItem? collectionItem;
  final CardModel? card;

  const CardDetailScreen({
    super.key,
    this.collectionItem,
    this.card,
  }) : assert(collectionItem != null || card != null,
            'Either collectionItem or card must be provided');

  @override
  State<CardDetailScreen> createState() => _CardDetailScreenState();
}

class _CardDetailScreenState extends State<CardDetailScreen> {
  late int _quantity;
  late String _condition;
  late bool _foil;
  bool _loading = false;
  bool _priceRefreshing = false;

  CardModel get _card => widget.collectionItem?.card ?? widget.card!;
  bool get _isCollectionItem => widget.collectionItem != null;

  static const List<String> _conditions = [
    'M',
    'NM',
    'EX',
    'GD',
    'LP',
    'PL',
    'PR',
  ];

  static const Map<String, String> _conditionLabels = {
    'M': 'Mint',
    'NM': 'Near Mint',
    'EX': 'Excellent',
    'GD': 'Good',
    'LP': 'Light Play',
    'PL': 'Played',
    'PR': 'Poor',
  };

  @override
  void initState() {
    super.initState();
    _quantity = widget.collectionItem?.quantity ?? 1;
    _condition = widget.collectionItem?.condition ?? 'NM';
    _foil = widget.collectionItem?.foil ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final mediaQuery = MediaQuery.of(context);
    final isWide = mediaQuery.size.width > 600;

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
        child: isWide
            ? _buildWideLayout(context)
            : _buildNarrowLayout(context),
      ),
    );
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

    return AspectRatio(
      aspectRatio: 2.5 / 3.5,
      child: Card(
        clipBehavior: Clip.antiAlias,
        elevation: 4,
        child: _card.imageUrl != null && _card.imageUrl!.isNotEmpty
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
            if (_foil) ...[
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    colors: [
                      Colors.purple.shade300,
                      Colors.blue.shade300,
                    ],
                  ),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  'FOIL',
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
          Expanded(
            child: Text(
              value,
              style: theme.textTheme.bodyMedium,
            ),
          ),
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
          value: _condition,
          decoration: InputDecoration(
            labelText: 'Condition',
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
            ),
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

        // Foil toggle
        SwitchListTile(
          title: const Text('Foil'),
          value: _foil,
          onChanged: (value) => setState(() => _foil = value),
          contentPadding: EdgeInsets.zero,
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
          value: _condition,
          decoration: InputDecoration(
            labelText: 'Condition',
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
            ),
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

        // Foil toggle
        SwitchListTile(
          title: const Text('Foil'),
          value: _foil,
          onChanged: (value) => setState(() => _foil = value),
          contentPadding: EdgeInsets.zero,
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
      await provider.updateItem(
        widget.collectionItem!.id,
        quantity: _quantity,
        condition: _condition,
        foil: _foil,
      );
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Card updated'),
          behavior: SnackBarBehavior.floating,
        ),
      );
      navigator.pop();
    } catch (e) {
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
        foil: _foil,
      );
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Added to collection'),
          behavior: SnackBarBehavior.floating,
        ),
      );
      navigator.pop();
    } catch (e) {
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
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Price refreshed'),
          behavior: SnackBarBehavior.floating,
        ),
      );
    } catch (e) {
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
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Card removed from collection'),
          behavior: SnackBarBehavior.floating,
        ),
      );
      navigator.pop();
    } catch (e) {
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
