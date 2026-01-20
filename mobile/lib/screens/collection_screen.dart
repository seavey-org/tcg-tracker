import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/grouped_collection.dart';
import '../providers/collection_provider.dart';
import '../utils/grid_utils.dart';
import '../widgets/collection_card.dart';
import '../widgets/collection_filters.dart';
import 'card_detail_screen.dart';

class CollectionScreen extends StatefulWidget {
  /// Callback to navigate to scan tab (called from empty state)
  final VoidCallback? onNavigateToScan;

  const CollectionScreen({super.key, this.onNavigateToScan});

  @override
  State<CollectionScreen> createState() => _CollectionScreenState();
}

class _CollectionScreenState extends State<CollectionScreen> {
  final TextEditingController _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    // Fetch grouped collection and load persisted filters on first load
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final provider = context.read<CollectionProvider>();
      // Load persisted filters
      provider.loadPersistedFilters();
      // Sync search controller with provider
      _searchController.text = provider.searchQuery;
      if (provider.allGroupedItems.isEmpty && !provider.loading) {
        provider.fetchGroupedCollection();
      }
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Collection'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _refreshPrices,
            tooltip: 'Refresh Prices',
          ),
          IconButton(
            icon: const Icon(Icons.settings),
            onPressed: () => Navigator.pushNamed(context, '/settings'),
          ),
        ],
      ),
      body: Consumer<CollectionProvider>(
        builder: (context, provider, child) {
          return Column(
            children: [
              // Filter and sort row
              _buildFilterSortRow(context, provider),
              // Content
              Expanded(child: _buildContent(context, provider)),
            ],
          );
        },
      ),
    );
  }

  Widget _buildFilterSortRow(
    BuildContext context,
    CollectionProvider provider,
  ) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Game and Sort dropdowns row
          Row(
            children: [
              // Game filter
              Expanded(
                child: DropdownButtonFormField<String?>(
                  value: provider.gameFilter,
                  decoration: InputDecoration(
                    labelText: 'Game',
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 8,
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8),
                    ),
                    isDense: true,
                  ),
                  items: const [
                    DropdownMenuItem(value: null, child: Text('All Games')),
                    DropdownMenuItem(value: 'mtg', child: Text('MTG')),
                    DropdownMenuItem(value: 'pokemon', child: Text('Pokemon')),
                  ],
                  onChanged: (value) => provider.setGameFilter(value),
                ),
              ),
              const SizedBox(width: 12),
              // Sort option
              Expanded(
                child: DropdownButtonFormField<SortOption>(
                  value: provider.sortOption,
                  decoration: InputDecoration(
                    labelText: 'Sort By',
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 8,
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8),
                    ),
                    isDense: true,
                  ),
                  items: const [
                    DropdownMenuItem(
                      value: SortOption.dateAdded,
                      child: Text('Recently Added'),
                    ),
                    DropdownMenuItem(
                      value: SortOption.name,
                      child: Text('Name'),
                    ),
                    DropdownMenuItem(
                      value: SortOption.value,
                      child: Text('Value'),
                    ),
                  ],
                  onChanged: (value) {
                    if (value != null) provider.setSortOption(value);
                  },
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          // Search bar
          TextField(
            controller: _searchController,
            decoration: InputDecoration(
              hintText: 'Search by card name or set...',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _searchController.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: () {
                        _searchController.clear();
                        provider.setSearchQuery('');
                      },
                    )
                  : null,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 16,
                vertical: 12,
              ),
              isDense: true,
            ),
            onChanged: (value) => provider.setSearchQuery(value),
          ),
          const SizedBox(height: 12),
          // Advanced filters
          CollectionFilters(
            filters: provider.advancedFilters,
            onFiltersChanged: provider.setAdvancedFilters,
            availablePrintings: provider.availablePrintings,
            availableSets: provider.availableSets,
            availableConditions: provider.availableConditions,
            availableRarities: provider.availableRarities,
          ),
        ],
      ),
    );
  }

  Widget _buildContent(BuildContext context, CollectionProvider provider) {
    if (provider.loading && provider.allGroupedItems.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.error != null && provider.allGroupedItems.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: 64,
              color: Theme.of(context).colorScheme.error,
            ),
            const SizedBox(height: 16),
            Text(
              'Failed to load collection',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              provider.error!,
              style: Theme.of(context).textTheme.bodySmall,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: () => provider.fetchGroupedCollection(),
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }

    // Show empty state if no items match filters, or if collection is truly empty
    if (provider.groupedItems.isEmpty) {
      return _buildEmptyState(context);
    }

    return RefreshIndicator(
      onRefresh: () async {
        await provider.fetchGroupedCollection();
        await provider.fetchStats();
      },
      child: LayoutBuilder(
        builder: (context, constraints) {
          // Calculate number of columns based on screen width
          final crossAxisCount = calculateColumns(constraints.maxWidth);
          final childAspectRatio = calculateGridAspectRatio(
            constraints.maxWidth,
          );

          return GridView.builder(
            padding: const EdgeInsets.all(12),
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: crossAxisCount,
              mainAxisSpacing: 12,
              crossAxisSpacing: 12,
              childAspectRatio: childAspectRatio,
            ),
            itemCount: provider.groupedItems.length,
            itemBuilder: (context, index) {
              final item = provider.groupedItems[index];
              return CollectionCard(
                key: ValueKey(item.card.id),
                groupedItem: item,
                onTap: () => _openCardDetail(context, item),
              );
            },
          );
        },
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    final theme = Theme.of(context);
    final provider = context.read<CollectionProvider>();
    final hasFilters = provider.hasActiveFilters;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              hasFilters
                  ? Icons.filter_list_off
                  : Icons.collections_bookmark_outlined,
              size: 80,
              color: theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
            ),
            const SizedBox(height: 24),
            Text(
              hasFilters
                  ? 'No cards match your filters'
                  : 'No cards in your collection',
              style: theme.textTheme.titleLarge?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              hasFilters
                  ? 'Try adjusting your search or filter criteria'
                  : 'Start scanning cards to add them to your collection',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            if (hasFilters)
              FilledButton.icon(
                onPressed: () {
                  _searchController.clear();
                  provider.clearAllFilters();
                },
                icon: const Icon(Icons.clear_all),
                label: const Text('Clear Filters'),
              )
            else
              FilledButton.icon(
                onPressed: widget.onNavigateToScan,
                icon: const Icon(Icons.camera_alt),
                label: const Text('Scan Cards'),
              ),
          ],
        ),
      ),
    );
  }

  void _openCardDetail(BuildContext context, GroupedCollectionItem item) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => CardDetailScreen(groupedItem: item),
      ),
    );
  }

  Future<void> _refreshPrices() async {
    final provider = context.read<CollectionProvider>();
    final messenger = ScaffoldMessenger.of(context);

    try {
      final updated = await provider.refreshAllPrices();
      if (!mounted) return;
      messenger.showSnackBar(
        SnackBar(
          content: Text('Queued $updated cards for price refresh'),
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
    }
  }
}
