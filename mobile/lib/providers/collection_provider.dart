import 'package:flutter/foundation.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../models/collection_stats.dart';
import '../models/grouped_collection.dart';
import '../models/price_status.dart';
import '../services/api_service.dart';

enum SortOption { dateAdded, name, value }

class CollectionProvider extends ChangeNotifier {
  final ApiService _apiService;

  CollectionProvider({ApiService? apiService})
    : _apiService = apiService ?? ApiService();

  // Collection state
  List<CollectionItem> _items = [];
  List<GroupedCollectionItem> _groupedItems = [];
  CollectionStats _stats = CollectionStats.empty();
  bool _loading = false;
  String? _error;

  // Last update result for showing split/merge feedback
  CollectionUpdateResponse? _lastUpdateResult;

  // Search state
  List<CardModel> _searchResults = [];
  bool _searchLoading = false;
  String? _searchError;

  // Price status state
  PriceStatus _priceStatus = PriceStatus.empty();
  bool _priceStatusLoading = false;

  // Filter and sort state
  String? _gameFilter;
  SortOption _sortOption = SortOption.dateAdded;

  // Getters
  List<CollectionItem> get items => _sortedAndFilteredItems;
  List<CollectionItem> get allItems => _items;
  List<GroupedCollectionItem> get groupedItems =>
      _sortedAndFilteredGroupedItems;
  List<GroupedCollectionItem> get allGroupedItems => _groupedItems;
  CollectionStats get stats => _stats;
  bool get loading => _loading;
  String? get error => _error;
  CollectionUpdateResponse? get lastUpdateResult => _lastUpdateResult;

  List<CardModel> get searchResults => _searchResults;
  bool get searchLoading => _searchLoading;
  String? get searchError => _searchError;

  PriceStatus get priceStatus => _priceStatus;
  bool get priceStatusLoading => _priceStatusLoading;

  String? get gameFilter => _gameFilter;
  SortOption get sortOption => _sortOption;

  int get totalCards => _stats.totalCards;
  double get totalValue => _stats.totalValue;

  List<CollectionItem> get mtgCards =>
      _items.where((i) => i.card.game == 'mtg').toList();
  List<CollectionItem> get pokemonCards =>
      _items.where((i) => i.card.game == 'pokemon').toList();

  /// Recent additions (last 12 items by date)
  List<CollectionItem> get recentAdditions {
    final sorted = List<CollectionItem>.from(_items)
      ..sort((a, b) => b.addedAt.compareTo(a.addedAt));
    return sorted.take(12).toList();
  }

  List<CollectionItem> get _sortedAndFilteredItems {
    var filtered = _items.toList();

    // Apply game filter
    if (_gameFilter != null && _gameFilter!.isNotEmpty) {
      filtered = filtered.where((i) => i.card.game == _gameFilter).toList();
    }

    // Apply sort
    switch (_sortOption) {
      case SortOption.dateAdded:
        filtered.sort((a, b) => b.addedAt.compareTo(a.addedAt));
        break;
      case SortOption.name:
        filtered.sort(
          (a, b) =>
              a.card.name.toLowerCase().compareTo(b.card.name.toLowerCase()),
        );
        break;
      case SortOption.value:
        filtered.sort((a, b) => b.totalValue.compareTo(a.totalValue));
        break;
    }

    return filtered;
  }

  List<GroupedCollectionItem> get _sortedAndFilteredGroupedItems {
    var filtered = _groupedItems.toList();

    // Apply game filter
    if (_gameFilter != null && _gameFilter!.isNotEmpty) {
      filtered = filtered.where((i) => i.card.game == _gameFilter).toList();
    }

    // Apply sort
    switch (_sortOption) {
      case SortOption.dateAdded:
        // Sort by most recent item in the group
        filtered.sort((a, b) {
          final aLatest = a.items.isNotEmpty
              ? a.items
                    .map((i) => i.addedAt)
                    .reduce((a, b) => a.isAfter(b) ? a : b)
              : DateTime(1970);
          final bLatest = b.items.isNotEmpty
              ? b.items
                    .map((i) => i.addedAt)
                    .reduce((a, b) => a.isAfter(b) ? a : b)
              : DateTime(1970);
          return bLatest.compareTo(aLatest);
        });
        break;
      case SortOption.name:
        filtered.sort(
          (a, b) =>
              a.card.name.toLowerCase().compareTo(b.card.name.toLowerCase()),
        );
        break;
      case SortOption.value:
        filtered.sort((a, b) => b.totalValue.compareTo(a.totalValue));
        break;
    }

    return filtered;
  }

  // Filter and sort methods
  void setGameFilter(String? game) {
    _gameFilter = game;
    notifyListeners();
  }

  void setSortOption(SortOption option) {
    _sortOption = option;
    notifyListeners();
  }

  // Collection methods
  Future<void> fetchCollection({String? game}) async {
    _loading = true;
    _error = null;
    notifyListeners();

    try {
      _items = await _apiService.getCollection(game: game);
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  /// Fetch collection items grouped by card
  Future<void> fetchGroupedCollection({String? game}) async {
    _loading = true;
    _error = null;
    notifyListeners();

    try {
      _groupedItems = await _apiService.getGroupedCollection(game: game);
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  Future<void> fetchStats() async {
    try {
      _stats = await _apiService.getStats();
      notifyListeners();
    } catch (e) {
      // Stats error is non-critical, just log it
      debugPrint('Failed to fetch stats: $e');
    }
  }

  Future<void> addToCollection(
    String cardId, {
    int quantity = 1,
    String condition = 'NM',
    PrintingType printing = PrintingType.normal,
  }) async {
    try {
      final item = await _apiService.addToCollection(
        cardId,
        quantity: quantity,
        condition: condition,
        printing: printing,
      );
      // Add to local list or update existing
      final existingIndex = _items.indexWhere((i) => i.id == item.id);
      if (existingIndex >= 0) {
        _items[existingIndex] = item;
      } else {
        _items.insert(0, item);
      }
      // Refresh stats
      await fetchStats();
      notifyListeners();
    } catch (e) {
      rethrow;
    }
  }

  /// Update a collection item
  /// Returns the update response which may indicate a split or merge occurred
  Future<CollectionUpdateResponse> updateItem(
    int id, {
    int? quantity,
    String? condition,
    PrintingType? printing,
    String? notes,
  }) async {
    try {
      final response = await _apiService.updateCollectionItem(
        id,
        quantity: quantity,
        condition: condition,
        printing: printing,
        notes: notes,
      );

      // Store the last update result for UI feedback
      _lastUpdateResult = response;

      // If a split or merge occurred, we need to refresh the full collection
      // because items may have been created or removed
      if (response.isSplit || response.isMerged) {
        await fetchCollection();
        await fetchGroupedCollection();
      } else {
        // Simple update, just update in place
        final index = _items.indexWhere((i) => i.id == id);
        if (index >= 0) {
          _items[index] = response.item;
        }
      }

      // Refresh stats if quantity or condition/printing changed
      if (quantity != null || condition != null || printing != null) {
        await fetchStats();
      }

      notifyListeners();
      return response;
    } catch (e) {
      rethrow;
    }
  }

  /// Clear the last update result (call after showing feedback to user)
  void clearLastUpdateResult() {
    _lastUpdateResult = null;
    notifyListeners();
  }

  Future<void> removeItem(int id) async {
    try {
      await _apiService.deleteCollectionItem(id);
      _items.removeWhere((i) => i.id == id);
      await fetchStats();
      notifyListeners();
    } catch (e) {
      rethrow;
    }
  }

  // Search methods
  Future<void> searchCards(String query, String game) async {
    _searchLoading = true;
    _searchError = null;
    notifyListeners();

    try {
      final result = await _apiService.searchCards(query, game);
      _searchResults = result.cards;
      _searchError = null;
    } catch (e) {
      _searchError = e.toString();
    } finally {
      _searchLoading = false;
      notifyListeners();
    }
  }

  void clearSearch() {
    _searchResults = [];
    _searchError = null;
    notifyListeners();
  }

  // Price methods
  Future<void> fetchPriceStatus() async {
    _priceStatusLoading = true;
    notifyListeners();

    try {
      _priceStatus = await _apiService.getPriceStatus();
    } catch (e) {
      debugPrint('Failed to fetch price status: $e');
    } finally {
      _priceStatusLoading = false;
      notifyListeners();
    }
  }

  Future<int> refreshAllPrices() async {
    try {
      final updated = await _apiService.refreshAllPrices();
      // Refresh collection to get updated prices
      await fetchCollection();
      await fetchPriceStatus();
      return updated;
    } catch (e) {
      rethrow;
    }
  }

  Future<CardModel> refreshCardPrice(String cardId) async {
    try {
      final card = await _apiService.refreshCardPrice(cardId);
      // Update the card in our local items
      for (var i = 0; i < _items.length; i++) {
        if (_items[i].cardId == cardId) {
          _items[i] = _items[i].copyWith(card: card);
        }
      }
      await fetchPriceStatus();
      notifyListeners();
      return card;
    } catch (e) {
      rethrow;
    }
  }

  /// Initialize the provider by loading data
  Future<void> initialize() async {
    await Future.wait([
      fetchCollection(),
      fetchGroupedCollection(),
      fetchStats(),
      fetchPriceStatus(),
    ]);
  }
}
