import 'card.dart';

/// A group of cards from the same set in grouped search results
class SetGroup {
  /// Set code (e.g., "swsh4", "MH2")
  final String setCode;

  /// Full set name (e.g., "Vivid Voltage")
  final String setName;

  /// Series name (e.g., "Sword & Shield") for Pokemon or set type for MTG
  final String? series;

  /// Release date in YYYY-MM-DD or YYYY/MM/DD format
  final String? releaseDate;

  /// URL to set symbol image (PNG for Pokemon, SVG for MTG)
  final String? symbolUrl;

  /// Number of cards matching the search in this set
  final int cardCount;

  /// The actual card objects in this set
  final List<CardModel> cards;

  SetGroup({
    required this.setCode,
    required this.setName,
    this.series,
    this.releaseDate,
    this.symbolUrl,
    required this.cardCount,
    required this.cards,
  });

  factory SetGroup.fromJson(Map<String, dynamic> json) {
    final cardsJson = json['cards'] as List<dynamic>? ?? [];
    return SetGroup(
      setCode: json['set_code'] ?? '',
      setName: json['set_name'] ?? '',
      series: json['series'],
      releaseDate: json['release_date'],
      symbolUrl: json['symbol_url'],
      cardCount: json['card_count'] ?? 0,
      cards: cardsJson
          .map((c) => CardModel.fromJson(c as Map<String, dynamic>))
          .toList(),
    );
  }

  /// Extract year from release date
  String? get releaseYear {
    if (releaseDate == null || releaseDate!.length < 4) return null;
    return releaseDate!.substring(0, 4);
  }

  /// Human-readable card count label
  String get cardCountLabel =>
      cardCount == 1 ? '1 card' : '$cardCount cards';
}

/// Result from grouped card search - cards organized by set
class GroupedSearchResult {
  /// The card name that was searched
  final String cardName;

  /// Sets containing matching cards
  final List<SetGroup> setGroups;

  /// Total number of sets with matches
  final int totalSets;

  GroupedSearchResult({
    required this.cardName,
    required this.setGroups,
    required this.totalSets,
  });

  factory GroupedSearchResult.fromJson(Map<String, dynamic> json) {
    final groupsJson = json['set_groups'] as List<dynamic>? ?? [];
    return GroupedSearchResult(
      cardName: json['card_name'] ?? '',
      setGroups: groupsJson
          .map((g) => SetGroup.fromJson(g as Map<String, dynamic>))
          .toList(),
      totalSets: json['total_sets'] ?? 0,
    );
  }

  /// Total number of cards across all sets
  int get totalCards =>
      setGroups.fold(0, (sum, group) => sum + group.cardCount);

  /// Returns true if any results were found
  bool get hasResults => setGroups.isNotEmpty;
}
