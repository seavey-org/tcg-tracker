import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/collection_provider.dart';
import '../widgets/collection_card.dart';
import 'card_detail_screen.dart';

class SearchScreen extends StatefulWidget {
  const SearchScreen({super.key});

  @override
  State<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends State<SearchScreen> {
  final _searchController = TextEditingController();
  String _selectedGame = 'pokemon';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Search Cards'),
        actions: [
          IconButton(
            icon: const Icon(Icons.settings),
            onPressed: () => Navigator.pushNamed(context, '/settings'),
          ),
        ],
      ),
      body: Column(
        children: [
          // Search bar
          Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                // Search input
                TextField(
                  controller: _searchController,
                  decoration: InputDecoration(
                    hintText: 'Search card name...',
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: _searchController.text.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.clear),
                            onPressed: _clearSearch,
                          )
                        : null,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  textInputAction: TextInputAction.search,
                  onSubmitted: (_) => _performSearch(),
                  onChanged: (_) => setState(() {}),
                ),
                const SizedBox(height: 12),

                // Game selector and search button
                Row(
                  children: [
                    // Game selector
                    Expanded(
                      child: SegmentedButton<String>(
                        segments: const [
                          ButtonSegment(
                            value: 'pokemon',
                            label: Text('Pokemon'),
                          ),
                          ButtonSegment(
                            value: 'mtg',
                            label: Text('MTG'),
                          ),
                        ],
                        selected: {_selectedGame},
                        onSelectionChanged: (selection) {
                          setState(() => _selectedGame = selection.first);
                        },
                      ),
                    ),
                    const SizedBox(width: 12),
                    // Search button
                    FilledButton(
                      onPressed: _searchController.text.trim().isNotEmpty
                          ? _performSearch
                          : null,
                      child: const Text('Search'),
                    ),
                  ],
                ),
              ],
            ),
          ),

          // Results
          Expanded(
            child: Consumer<CollectionProvider>(
              builder: (context, provider, child) {
                return _buildResults(context, provider);
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildResults(BuildContext context, CollectionProvider provider) {
    final theme = Theme.of(context);

    if (provider.searchLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.searchError != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: 64,
              color: theme.colorScheme.error,
            ),
            const SizedBox(height: 16),
            Text(
              'Search failed',
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              provider.searchError!,
              style: theme.textTheme.bodySmall,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _performSearch,
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }

    if (provider.searchResults.isEmpty) {
      return _buildEmptyState(context);
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final crossAxisCount = _calculateColumns(constraints.maxWidth);
        final childAspectRatio = _calculateAspectRatio(constraints.maxWidth);

        return Column(
          children: [
            // Results count
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: Row(
                children: [
                  Text(
                    '${provider.searchResults.length} results',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const Spacer(),
                  TextButton(
                    onPressed: _clearSearch,
                    child: const Text('Clear'),
                  ),
                ],
              ),
            ),

            // Grid
            Expanded(
              child: GridView.builder(
                padding: const EdgeInsets.all(12),
                gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: crossAxisCount,
                  mainAxisSpacing: 12,
                  crossAxisSpacing: 12,
                  childAspectRatio: childAspectRatio,
                ),
                itemCount: provider.searchResults.length,
                itemBuilder: (context, index) {
                  final card = provider.searchResults[index];
                  return CollectionCard(
                    card: card,
                    onTap: () => _openCardDetail(context, card),
                  );
                },
              ),
            ),
          ],
        );
      },
    );
  }

  int _calculateColumns(double width) {
    if (width < 400) return 2;
    if (width < 600) return 3;
    if (width < 900) return 4;
    return 5;
  }

  double _calculateAspectRatio(double width) {
    if (width < 400) return 0.55;
    if (width < 600) return 0.58;
    return 0.6;
  }

  Widget _buildEmptyState(BuildContext context) {
    final theme = Theme.of(context);
    final hasSearched = _searchController.text.isNotEmpty;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              hasSearched ? Icons.search_off : Icons.search,
              size: 80,
              color: theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
            ),
            const SizedBox(height: 24),
            Text(
              hasSearched ? 'No cards found' : 'Search for cards',
              style: theme.textTheme.titleLarge?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              hasSearched
                  ? 'Try a different search term'
                  : 'Enter a card name to search',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  void _performSearch() {
    final query = _searchController.text.trim();
    if (query.isEmpty) return;

    FocusScope.of(context).unfocus();
    context.read<CollectionProvider>().searchCards(query, _selectedGame);
  }

  void _clearSearch() {
    _searchController.clear();
    context.read<CollectionProvider>().clearSearch();
    setState(() {});
  }

  void _openCardDetail(BuildContext context, dynamic card) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => CardDetailScreen(card: card),
      ),
    );
  }
}
