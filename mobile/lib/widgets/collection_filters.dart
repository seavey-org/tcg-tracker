import 'package:flutter/material.dart';
import '../models/collection_item.dart';

/// Filter state for collection
class CollectionFilterState {
  final Set<String> printings;
  final Set<String> sets;
  final Set<String> conditions;
  final Set<String> rarities;

  const CollectionFilterState({
    this.printings = const {},
    this.sets = const {},
    this.conditions = const {},
    this.rarities = const {},
  });

  CollectionFilterState copyWith({
    Set<String>? printings,
    Set<String>? sets,
    Set<String>? conditions,
    Set<String>? rarities,
  }) {
    return CollectionFilterState(
      printings: printings ?? this.printings,
      sets: sets ?? this.sets,
      conditions: conditions ?? this.conditions,
      rarities: rarities ?? this.rarities,
    );
  }

  int get activeCount =>
      printings.length + sets.length + conditions.length + rarities.length;

  bool get hasActiveFilters => activeCount > 0;

  CollectionFilterState clear() => const CollectionFilterState();

  /// Convert to map for persistence
  Map<String, List<String>> toJson() => {
    'printings': printings.toList(),
    'sets': sets.toList(),
    'conditions': conditions.toList(),
    'rarities': rarities.toList(),
  };

  /// Create from persisted map
  factory CollectionFilterState.fromJson(Map<String, dynamic> json) {
    return CollectionFilterState(
      printings: Set<String>.from(json['printings'] ?? []),
      sets: Set<String>.from(json['sets'] ?? []),
      conditions: Set<String>.from(json['conditions'] ?? []),
      rarities: Set<String>.from(json['rarities'] ?? []),
    );
  }
}

/// Set info for filter display
class SetInfo {
  final String code;
  final String name;

  const SetInfo({required this.code, required this.name});

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is SetInfo &&
          runtimeType == other.runtimeType &&
          code == other.code;

  @override
  int get hashCode => code.hashCode;
}

/// Collapsible filter panel for collection
class CollectionFilters extends StatefulWidget {
  final CollectionFilterState filters;
  final ValueChanged<CollectionFilterState> onFiltersChanged;
  final List<PrintingType> availablePrintings;
  final List<SetInfo> availableSets;
  final List<String> availableConditions;
  final List<String> availableRarities;

  const CollectionFilters({
    super.key,
    required this.filters,
    required this.onFiltersChanged,
    required this.availablePrintings,
    required this.availableSets,
    required this.availableConditions,
    required this.availableRarities,
  });

  @override
  State<CollectionFilters> createState() => _CollectionFiltersState();
}

class _CollectionFiltersState extends State<CollectionFilters> {
  bool _expanded = false;
  final TextEditingController _setSearchController = TextEditingController();
  String _setSearchQuery = '';

  @override
  void dispose() {
    _setSearchController.dispose();
    super.dispose();
  }

  List<SetInfo> get _filteredSets {
    if (_setSearchQuery.isEmpty) return widget.availableSets;
    final query = _setSearchQuery.toLowerCase();
    return widget.availableSets
        .where(
          (s) =>
              s.name.toLowerCase().contains(query) ||
              s.code.toLowerCase().contains(query),
        )
        .toList();
  }

  void _togglePrinting(String value) {
    final newPrintings = Set<String>.from(widget.filters.printings);
    if (newPrintings.contains(value)) {
      newPrintings.remove(value);
    } else {
      newPrintings.add(value);
    }
    widget.onFiltersChanged(widget.filters.copyWith(printings: newPrintings));
  }

  void _toggleSet(String code) {
    final newSets = Set<String>.from(widget.filters.sets);
    if (newSets.contains(code)) {
      newSets.remove(code);
    } else {
      newSets.add(code);
    }
    widget.onFiltersChanged(widget.filters.copyWith(sets: newSets));
  }

  void _toggleCondition(String value) {
    final newConditions = Set<String>.from(widget.filters.conditions);
    if (newConditions.contains(value)) {
      newConditions.remove(value);
    } else {
      newConditions.add(value);
    }
    widget.onFiltersChanged(widget.filters.copyWith(conditions: newConditions));
  }

  void _toggleRarity(String value) {
    final newRarities = Set<String>.from(widget.filters.rarities);
    if (newRarities.contains(value)) {
      newRarities.remove(value);
    } else {
      newRarities.add(value);
    }
    widget.onFiltersChanged(widget.filters.copyWith(rarities: newRarities));
  }

  void _clearAll() {
    _setSearchController.clear();
    setState(() {
      _setSearchQuery = '';
    });
    widget.onFiltersChanged(widget.filters.clear());
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final activeCount = widget.filters.activeCount;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Filter toggle button
        Row(
          children: [
            ActionChip(
              avatar: Icon(
                _expanded ? Icons.expand_less : Icons.expand_more,
                size: 18,
              ),
              label: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text('Filters'),
                  if (activeCount > 0) ...[
                    const SizedBox(width: 6),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: colorScheme.primary,
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Text(
                        '$activeCount',
                        style: TextStyle(
                          color: colorScheme.onPrimary,
                          fontSize: 12,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
              backgroundColor: activeCount > 0
                  ? colorScheme.primaryContainer
                  : colorScheme.surfaceContainerHighest,
              onPressed: () => setState(() => _expanded = !_expanded),
            ),
            if (activeCount > 0) ...[
              const SizedBox(width: 8),
              IconButton(
                icon: const Icon(Icons.clear, size: 20),
                onPressed: _clearAll,
                tooltip: 'Clear all filters',
                visualDensity: VisualDensity.compact,
              ),
            ],
          ],
        ),

        // Collapsible panel
        AnimatedCrossFade(
          duration: const Duration(milliseconds: 200),
          crossFadeState: _expanded
              ? CrossFadeState.showSecond
              : CrossFadeState.showFirst,
          firstChild: const SizedBox.shrink(),
          secondChild: _buildFilterPanel(context),
        ),
      ],
    );
  }

  Widget _buildFilterPanel(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      margin: const EdgeInsets.only(top: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Printing filter
          if (widget.availablePrintings.isNotEmpty) ...[
            _buildFilterSection(
              context,
              label: 'Printing',
              children: widget.availablePrintings.map((printing) {
                final isSelected = widget.filters.printings.contains(
                  printing.value,
                );
                return FilterChip(
                  label: Text(printing.value),
                  selected: isSelected,
                  onSelected: (_) => _togglePrinting(printing.value),
                );
              }).toList(),
            ),
            const SizedBox(height: 16),
          ],

          // Set filter with search
          if (widget.availableSets.isNotEmpty) ...[
            _buildFilterSection(
              context,
              label: 'Set',
              children: [
                // Search input
                SizedBox(
                  width: double.infinity,
                  child: TextField(
                    controller: _setSearchController,
                    decoration: InputDecoration(
                      hintText: 'Search sets...',
                      isDense: true,
                      prefixIcon: const Icon(Icons.search, size: 20),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 8,
                      ),
                    ),
                    onChanged: (value) =>
                        setState(() => _setSearchQuery = value),
                  ),
                ),
                const SizedBox(height: 8),
                // Set chips in scrollable container
                ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 100),
                  child: SingleChildScrollView(
                    child: Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: _filteredSets.map((set) {
                        final isSelected = widget.filters.sets.contains(
                          set.code,
                        );
                        return FilterChip(
                          label: Text(set.name),
                          selected: isSelected,
                          onSelected: (_) => _toggleSet(set.code),
                          tooltip: set.code,
                        );
                      }).toList(),
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
          ],

          // Condition filter
          if (widget.availableConditions.isNotEmpty) ...[
            _buildFilterSection(
              context,
              label: 'Condition',
              children: widget.availableConditions.map((condition) {
                final isSelected = widget.filters.conditions.contains(
                  condition,
                );
                return FilterChip(
                  label: Text(condition),
                  selected: isSelected,
                  onSelected: (_) => _toggleCondition(condition),
                  tooltip: _conditionLabel(condition),
                );
              }).toList(),
            ),
            const SizedBox(height: 16),
          ],

          // Rarity filter
          if (widget.availableRarities.isNotEmpty) ...[
            _buildFilterSection(
              context,
              label: 'Rarity',
              children: widget.availableRarities.map((rarity) {
                final isSelected = widget.filters.rarities.contains(rarity);
                return FilterChip(
                  label: Text(rarity),
                  selected: isSelected,
                  onSelected: (_) => _toggleRarity(rarity),
                );
              }).toList(),
            ),
          ],

          // Clear all button
          if (widget.filters.hasActiveFilters) ...[
            const SizedBox(height: 16),
            const Divider(),
            const SizedBox(height: 8),
            TextButton.icon(
              onPressed: _clearAll,
              icon: const Icon(Icons.clear_all),
              label: const Text('Clear All Filters'),
              style: TextButton.styleFrom(foregroundColor: colorScheme.error),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildFilterSection(
    BuildContext context, {
    required String label,
    required List<Widget> children,
  }) {
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: theme.textTheme.labelLarge?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 8),
        Wrap(spacing: 8, runSpacing: 8, children: children),
      ],
    );
  }

  String _conditionLabel(String condition) {
    const labels = {
      'M': 'Mint',
      'NM': 'Near Mint',
      'EX': 'Excellent',
      'GD': 'Good',
      'LP': 'Light Play',
      'PL': 'Played',
      'PR': 'Poor',
    };
    return labels[condition] ?? condition;
  }
}
