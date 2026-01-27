import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:provider/provider.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../models/grouped_search_result.dart';
import '../providers/collection_provider.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../widgets/admin_key_dialog.dart';

/// Sort options for grouped search results
enum SearchSortOrder {
  releaseDate('release_date', 'Newest First'),
  releaseDateAsc('release_date_asc', 'Oldest First'),
  name('name', 'Alphabetical'),
  cards('cards', 'Most Cards');

  final String value;
  final String label;
  const SearchSortOrder(this.value, this.label);
}

/// Screen for reassigning a collection item to a different card
/// Uses 2-phase selection: Set list -> Card list within set
class ReassignCardScreen extends StatefulWidget {
  final CollectionItem item;
  final ApiService? apiService;

  const ReassignCardScreen({super.key, required this.item, this.apiService});

  @override
  State<ReassignCardScreen> createState() => _ReassignCardScreenState();
}

class _ReassignCardScreenState extends State<ReassignCardScreen> {
  late final ApiService _apiService;
  final _searchController = TextEditingController();

  // UI state
  String _selectedGame = 'pokemon';
  SearchSortOrder _sortOrder = SearchSortOrder.releaseDate;
  bool _searching = false;
  String? _searchError;
  bool _reassigning = false;

  // Data state
  GroupedSearchResult? _searchResult;
  SetGroup? _selectedSet; // Phase 2: selected set
  CardModel? _selectedCard; // Final selection

  @override
  void initState() {
    super.initState();
    _apiService = widget.apiService ?? ApiService();
    _selectedGame = widget.item.card.game;
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _performSearch() async {
    final query = _searchController.text.trim();
    if (query.length < 2) return;

    setState(() {
      _searching = true;
      _searchError = null;
      _selectedSet = null;
      _selectedCard = null;
    });

    try {
      final result = await _apiService.searchCardsGrouped(
        query,
        _selectedGame,
        sort: _sortOrder.value,
      );
      if (mounted) {
        setState(() {
          _searchResult = result;
          _searching = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _searchError = e.toString();
          _searching = false;
        });
      }
    }
  }

  void _selectSet(SetGroup set) {
    setState(() {
      _selectedSet = set;
      _selectedCard = null;
    });
  }

  void _backToSetList() {
    setState(() {
      _selectedSet = null;
      _selectedCard = null;
    });
  }

  void _selectCard(CardModel card) {
    setState(() {
      _selectedCard = card;
    });
  }

  Future<void> _confirmReassign() async {
    if (_selectedCard == null) return;

    setState(() => _reassigning = true);

    try {
      final provider = context.read<CollectionProvider>();
      await provider.updateItem(widget.item.id, cardId: _selectedCard!.id);

      if (!mounted) return;

      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Reassigned to ${_selectedCard!.name}'),
          backgroundColor: Colors.green,
        ),
      );

      Navigator.pop(context, true); // Return true to indicate success
    } on AuthRequiredException {
      if (!mounted) return;
      setState(() => _reassigning = false);
      final success = await AdminKeyDialog.show(context, _apiService);
      if (success && mounted) {
        _confirmReassign();
      }
    } catch (e) {
      if (!mounted) return;
      setState(() => _reassigning = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Error: $e'),
          backgroundColor: Theme.of(context).colorScheme.error,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(
        title: Text(
          _selectedSet != null ? _selectedSet!.setName : 'Reassign Card',
        ),
        leading: _selectedSet != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: _backToSetList,
              )
            : null,
      ),
      body: Column(
        children: [
          // Current card info banner
          Container(
            padding: const EdgeInsets.all(12),
            color: colorScheme.surfaceContainerHighest,
            child: Row(
              children: [
                // Scanned image thumbnail
                if (widget.item.scannedImagePath != null)
                  FutureBuilder<String>(
                    future: _apiService.getServerUrl(),
                    builder: (context, snapshot) {
                      if (!snapshot.hasData) {
                        return const SizedBox(width: 50, height: 70);
                      }
                      return ClipRRect(
                        borderRadius: BorderRadius.circular(4),
                        child: SizedBox(
                          width: 50,
                          height: 70,
                          child: CachedNetworkImage(
                            imageUrl:
                                '${snapshot.data}/images/scanned/${widget.item.scannedImagePath}',
                            fit: BoxFit.cover,
                            placeholder: (context, url) =>
                                Container(color: colorScheme.surface),
                            errorWidget: (context, url, error) => Container(
                              color: colorScheme.surface,
                              child: const Icon(Icons.image),
                            ),
                          ),
                        ),
                      );
                    },
                  )
                else
                  ClipRRect(
                    borderRadius: BorderRadius.circular(4),
                    child: SizedBox(
                      width: 50,
                      height: 70,
                      child: widget.item.card.imageUrl != null
                          ? CachedNetworkImage(
                              imageUrl: widget.item.card.imageUrl!,
                              fit: BoxFit.cover,
                            )
                          : Container(
                              color: colorScheme.surface,
                              child: const Icon(Icons.image),
                            ),
                    ),
                  ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Currently assigned to:',
                        style: theme.textTheme.labelSmall?.copyWith(
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                      Text(
                        widget.item.card.name,
                        style: theme.textTheme.titleSmall?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      Text(
                        widget.item.card.displaySet,
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

          // Search controls (only in Phase 1)
          if (_selectedSet == null) ...[
            Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                children: [
                  // Game toggle
                  SegmentedButton<String>(
                    segments: const [
                      ButtonSegment(value: 'pokemon', label: Text('Pokemon')),
                      ButtonSegment(value: 'mtg', label: Text('MTG')),
                    ],
                    selected: {_selectedGame},
                    onSelectionChanged: (selection) {
                      setState(() {
                        _selectedGame = selection.first;
                        _searchResult = null;
                        _selectedSet = null;
                        _selectedCard = null;
                      });
                    },
                  ),
                  const SizedBox(height: 12),

                  // Search input
                  TextField(
                    controller: _searchController,
                    decoration: InputDecoration(
                      hintText: 'Search for card by name...',
                      prefixIcon: const Icon(Icons.search),
                      suffixIcon: _searchController.text.isNotEmpty
                          ? IconButton(
                              icon: const Icon(Icons.clear),
                              onPressed: () {
                                _searchController.clear();
                                setState(() {
                                  _searchResult = null;
                                  _selectedSet = null;
                                  _selectedCard = null;
                                });
                              },
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
                ],
              ),
            ),
          ],

          // Content area
          Expanded(child: _buildContent(context)),

          // Selected card preview and confirm button
          if (_selectedCard != null)
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: colorScheme.primaryContainer,
                border: Border(top: BorderSide(color: colorScheme.outline)),
              ),
              child: SafeArea(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Row(
                      children: [
                        ClipRRect(
                          borderRadius: BorderRadius.circular(4),
                          child: SizedBox(
                            width: 40,
                            height: 56,
                            child: _selectedCard!.imageUrl != null
                                ? CachedNetworkImage(
                                    imageUrl: _selectedCard!.imageUrl!,
                                    fit: BoxFit.cover,
                                  )
                                : Container(
                                    color: colorScheme.surface,
                                    child: const Icon(Icons.image, size: 20),
                                  ),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                _selectedCard!.name,
                                style: theme.textTheme.titleSmall?.copyWith(
                                  fontWeight: FontWeight.bold,
                                  color: colorScheme.onPrimaryContainer,
                                ),
                              ),
                              Text(
                                _selectedCard!.displaySet,
                                style: theme.textTheme.bodySmall?.copyWith(
                                  color: colorScheme.onPrimaryContainer,
                                ),
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                          icon: const Icon(Icons.close),
                          onPressed: () => setState(() => _selectedCard = null),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton(
                        onPressed: _reassigning ? null : _confirmReassign,
                        child: _reassigning
                            ? const SizedBox(
                                width: 20,
                                height: 20,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : const Text('Reassign to Selected Card'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildContent(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    // Loading state
    if (_searching) {
      return const Center(child: CircularProgressIndicator());
    }

    // Error state
    if (_searchError != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: colorScheme.error),
            const SizedBox(height: 16),
            Text('Search failed', style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              _searchError!,
              style: theme.textTheme.bodySmall,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            FilledButton(onPressed: _performSearch, child: const Text('Retry')),
          ],
        ),
      );
    }

    // Phase 2: Card list within selected set
    if (_selectedSet != null) {
      return _buildCardList(context, _selectedSet!);
    }

    // Phase 1: Set list or empty state
    if (_searchResult == null || !_searchResult!.hasResults) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                _searchController.text.length < 2
                    ? Icons.search
                    : Icons.search_off,
                size: 64,
                color: colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
              ),
              const SizedBox(height: 16),
              Text(
                _searchController.text.length < 2
                    ? 'Search for a card'
                    : 'No cards found',
                style: theme.textTheme.titleMedium?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                _searchController.text.length < 2
                    ? 'Enter at least 2 characters to search'
                    : 'Try a different search term',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      );
    }

    return _buildSetList(context, _searchResult!);
  }

  Widget _buildSetList(BuildContext context, GroupedSearchResult result) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Column(
      children: [
        // Header with count and sort
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            children: [
              Text(
                '${result.totalCards} cards in ${result.totalSets} sets',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
              const Spacer(),
              // Sort dropdown
              DropdownButton<SearchSortOrder>(
                value: _sortOrder,
                underline: const SizedBox(),
                isDense: true,
                items: SearchSortOrder.values.map((order) {
                  return DropdownMenuItem(
                    value: order,
                    child: Text(order.label, style: theme.textTheme.bodySmall),
                  );
                }).toList(),
                onChanged: (value) {
                  if (value != null) {
                    setState(() => _sortOrder = value);
                    _performSearch();
                  }
                },
              ),
            ],
          ),
        ),

        // Set list
        Expanded(
          child: ListView.builder(
            itemCount: result.setGroups.length,
            itemBuilder: (context, index) {
              final setGroup = result.setGroups[index];
              return _buildSetTile(context, setGroup);
            },
          ),
        ),
      ],
    );
  }

  Widget _buildSetTile(BuildContext context, SetGroup setGroup) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: ListTile(
        leading: _buildSetIcon(setGroup),
        title: Text(setGroup.setName, style: theme.textTheme.titleSmall),
        subtitle: Text(
          [
            setGroup.series,
            if (setGroup.releaseYear != null) '(${setGroup.releaseYear})',
          ].whereType<String>().join(' '),
          style: theme.textTheme.bodySmall?.copyWith(
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              setGroup.cardCountLabel,
              style: theme.textTheme.labelMedium?.copyWith(
                color: colorScheme.primary,
              ),
            ),
            const SizedBox(width: 8),
            const Icon(Icons.chevron_right),
          ],
        ),
        onTap: () => _selectSet(setGroup),
      ),
    );
  }

  Widget _buildSetIcon(SetGroup setGroup) {
    if (setGroup.symbolUrl != null && setGroup.symbolUrl!.isNotEmpty) {
      return SizedBox(
        width: 40,
        height: 40,
        child: CachedNetworkImage(
          imageUrl: setGroup.symbolUrl!,
          fit: BoxFit.contain,
          placeholder: (context, url) =>
              _buildFallbackSetIcon(setGroup.setCode),
          errorWidget: (context, url, error) =>
              _buildFallbackSetIcon(setGroup.setCode),
        ),
      );
    }
    return _buildFallbackSetIcon(setGroup.setCode);
  }

  Widget _buildFallbackSetIcon(String setCode) {
    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(4),
        color: Colors.grey.shade200,
      ),
      child: Center(
        child: Text(
          setCode.toUpperCase().substring(0, setCode.length.clamp(0, 4)),
          style: const TextStyle(fontSize: 10, fontWeight: FontWeight.bold),
        ),
      ),
    );
  }

  Widget _buildCardList(BuildContext context, SetGroup setGroup) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Column(
      children: [
        // Set header
        Container(
          padding: const EdgeInsets.all(12),
          color: colorScheme.surfaceContainerHighest,
          child: Row(
            children: [
              _buildSetIcon(setGroup),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      setGroup.setName,
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    Text(
                      'Select a card',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
              Text(
                setGroup.cardCountLabel,
                style: theme.textTheme.labelMedium?.copyWith(
                  color: colorScheme.primary,
                ),
              ),
            ],
          ),
        ),

        // Card grid
        Expanded(
          child: GridView.builder(
            padding: const EdgeInsets.all(12),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              mainAxisSpacing: 8,
              crossAxisSpacing: 8,
              childAspectRatio: 0.6,
            ),
            itemCount: setGroup.cards.length,
            itemBuilder: (context, index) {
              final card = setGroup.cards[index];
              final isSelected = _selectedCard?.id == card.id;

              return GestureDetector(
                onTap: () => _selectCard(card),
                child: Container(
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(8),
                    border: isSelected
                        ? Border.all(color: colorScheme.primary, width: 3)
                        : null,
                  ),
                  child: Column(
                    children: [
                      Expanded(
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(6),
                          child: card.imageUrl != null
                              ? CachedNetworkImage(
                                  imageUrl: card.imageUrl!,
                                  fit: BoxFit.cover,
                                  placeholder: (context, url) => Container(
                                    color: colorScheme.surfaceContainerHighest,
                                    child: const Center(
                                      child: CircularProgressIndicator(
                                        strokeWidth: 2,
                                      ),
                                    ),
                                  ),
                                  errorWidget: (context, url, error) =>
                                      Container(
                                        color:
                                            colorScheme.surfaceContainerHighest,
                                        child: const Icon(Icons.broken_image),
                                      ),
                                )
                              : Container(
                                  color: colorScheme.surfaceContainerHighest,
                                  child: const Icon(Icons.image),
                                ),
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        card.name,
                        style: theme.textTheme.labelSmall,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      if (card.cardNumber != null)
                        Text(
                          '#${card.cardNumber}',
                          style: theme.textTheme.labelSmall?.copyWith(
                            color: colorScheme.onSurfaceVariant,
                          ),
                        ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}
