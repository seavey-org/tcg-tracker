/// Information about a card set for browsing and 2-phase selection
class SetInfo {
  /// Set code (e.g., "swsh4", "MH2")
  final String id;

  /// Full set name (e.g., "Vivid Voltage", "Modern Horizons 2")
  final String name;

  /// Series name (e.g., "Sword & Shield") or set type for MTG (e.g., "expansion")
  final String series;

  /// Release date in YYYY-MM-DD or YYYY/MM/DD format
  final String releaseDate;

  /// Total number of cards in the set
  final int totalCards;

  /// URL to set symbol image (PNG for Pokemon, SVG for MTG)
  final String? symbolUrl;

  /// URL to set logo image (Pokemon only)
  final String? logoUrl;

  SetInfo({
    required this.id,
    required this.name,
    required this.series,
    required this.releaseDate,
    required this.totalCards,
    this.symbolUrl,
    this.logoUrl,
  });

  factory SetInfo.fromJson(Map<String, dynamic> json) {
    return SetInfo(
      id: json['id'] ?? '',
      name: json['name'] ?? '',
      series: json['series'] ?? '',
      releaseDate: json['release_date'] ?? '',
      totalCards: json['total_cards'] ?? 0,
      symbolUrl: json['symbol_url'],
      logoUrl: json['logo_url'],
    );
  }

  /// Extract year from release date (YYYY-MM-DD or YYYY/MM/DD format)
  String? get releaseYear {
    if (releaseDate.isEmpty || releaseDate.length < 4) return null;
    return releaseDate.substring(0, 4);
  }
}

/// Result from listing sets
class SetListResult {
  final List<SetInfo> sets;

  SetListResult({required this.sets});

  factory SetListResult.fromJson(Map<String, dynamic> json) {
    final setsJson = json['sets'] as List<dynamic>? ?? [];
    return SetListResult(
      sets: setsJson
          .map((s) => SetInfo.fromJson(s as Map<String, dynamic>))
          .toList(),
    );
  }
}
