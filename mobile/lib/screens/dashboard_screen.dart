import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/collection_provider.dart';
import '../utils/grid_utils.dart';
import '../widgets/stats_card.dart';
import '../widgets/collection_card.dart';
import 'card_detail_screen.dart';

class DashboardScreen extends StatefulWidget {
  /// Callback to navigate to collection tab
  final VoidCallback? onNavigateToCollection;

  const DashboardScreen({super.key, this.onNavigateToCollection});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final provider = context.read<CollectionProvider>();
      // Refresh data
      provider.fetchStats();
      provider.fetchPriceStatus();
      if (provider.allItems.isEmpty) {
        provider.fetchCollection();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Dashboard'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _refresh,
            tooltip: 'Refresh',
          ),
          IconButton(
            icon: const Icon(Icons.settings),
            onPressed: () => Navigator.pushNamed(context, '/settings'),
          ),
        ],
      ),
      body: Consumer<CollectionProvider>(
        builder: (context, provider, child) {
          return RefreshIndicator(
            onRefresh: () async {
              await Future.wait([
                provider.fetchStats(),
                provider.fetchPriceStatus(),
                provider.fetchCollection(),
              ]);
            },
            child: SingleChildScrollView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  // Stats grid
                  _buildStatsGrid(context, provider),
                  const SizedBox(height: 16),

                  // Price API quota
                  PriceQuotaCard(
                    remaining: provider.priceStatus.remaining,
                    dailyLimit: provider.priceStatus.dailyLimit,
                    resetTime: provider.priceStatus.resetTimeDisplay,
                    loading: provider.priceStatusLoading,
                  ),
                  const SizedBox(height: 24),

                  // Recent additions
                  _buildRecentSection(context, provider),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildStatsGrid(BuildContext context, CollectionProvider provider) {
    final stats = provider.stats;

    return LayoutBuilder(
      builder: (context, constraints) {
        final isWide = constraints.maxWidth > 500;

        if (isWide) {
          // 2x2 grid on wider screens
          return Column(
            children: [
              Row(
                children: [
                  Expanded(
                    child: StatsCard(
                      title: 'Total Cards',
                      value: '${stats.totalCards}',
                      icon: Icons.collections_bookmark,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: StatsCard(
                      title: 'Unique Cards',
                      value: '${stats.uniqueCards}',
                      icon: Icons.style,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: StatsCard(
                      title: 'Collection Value',
                      value: stats.displayTotalValue,
                      icon: Icons.attach_money,
                      iconColor: Colors.green,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: GameBreakdownCard(
                      mtgCount: stats.mtgCards,
                      pokemonCount: stats.pokemonCards,
                      mtgValue: stats.displayMtgValue,
                      pokemonValue: stats.displayPokemonValue,
                    ),
                  ),
                ],
              ),
            ],
          );
        }

        // Vertical list on narrow screens
        return Column(
          children: [
            StatsCard(
              title: 'Total Cards',
              value: '${stats.totalCards}',
              icon: Icons.collections_bookmark,
            ),
            const SizedBox(height: 12),
            StatsCard(
              title: 'Unique Cards',
              value: '${stats.uniqueCards}',
              icon: Icons.style,
            ),
            const SizedBox(height: 12),
            StatsCard(
              title: 'Collection Value',
              value: stats.displayTotalValue,
              icon: Icons.attach_money,
              iconColor: Colors.green,
            ),
            const SizedBox(height: 12),
            GameBreakdownCard(
              mtgCount: stats.mtgCards,
              pokemonCount: stats.pokemonCards,
              mtgValue: stats.displayMtgValue,
              pokemonValue: stats.displayPokemonValue,
            ),
          ],
        );
      },
    );
  }

  Widget _buildRecentSection(BuildContext context, CollectionProvider provider) {
    final theme = Theme.of(context);
    final recentItems = provider.recentAdditions;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Recent Additions',
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const Spacer(),
            if (recentItems.isNotEmpty)
              TextButton(
                onPressed: widget.onNavigateToCollection,
                child: const Text('View All'),
              ),
          ],
        ),
        const SizedBox(height: 12),

        if (provider.loading && recentItems.isEmpty)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(32),
              child: CircularProgressIndicator(),
            ),
          )
        else if (recentItems.isEmpty)
          _buildEmptyRecent(context)
        else
          _buildRecentGrid(context, recentItems),
      ],
    );
  }

  Widget _buildEmptyRecent(BuildContext context) {
    final theme = Theme.of(context);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          children: [
            Icon(
              Icons.inbox_outlined,
              size: 48,
              color: theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
            ),
            const SizedBox(height: 16),
            Text(
              'No recent additions',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Start scanning cards to add them to your collection',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildRecentGrid(BuildContext context, List recentItems) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final crossAxisCount = calculateColumns(constraints.maxWidth);
        final childAspectRatio = calculateGridAspectRatio(constraints.maxWidth);
        final itemCount = recentItems.length.clamp(0, 6);

        return GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: crossAxisCount,
            mainAxisSpacing: 12,
            crossAxisSpacing: 12,
            childAspectRatio: childAspectRatio,
          ),
          itemCount: itemCount,
          itemBuilder: (context, index) {
            final item = recentItems[index];
            return CollectionCard(
              collectionItem: item,
              onTap: () => _openCardDetail(context, item),
            );
          },
        );
      },
    );
  }

  void _openCardDetail(BuildContext context, dynamic item) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => CardDetailScreen(collectionItem: item),
      ),
    );
  }

  Future<void> _refresh() async {
    final provider = context.read<CollectionProvider>();
    await Future.wait([
      provider.fetchStats(),
      provider.fetchPriceStatus(),
      provider.fetchCollection(),
    ]);
  }
}
