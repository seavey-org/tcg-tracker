class CollectionStats {
  final int totalCards;
  final int uniqueCards;
  final double totalValue;
  final int mtgCards;
  final int pokemonCards;
  final double mtgValue;
  final double pokemonValue;

  CollectionStats({
    required this.totalCards,
    required this.uniqueCards,
    required this.totalValue,
    required this.mtgCards,
    required this.pokemonCards,
    required this.mtgValue,
    required this.pokemonValue,
  });

  factory CollectionStats.fromJson(Map<String, dynamic> json) {
    return CollectionStats(
      totalCards: json['total_cards'] ?? 0,
      uniqueCards: json['unique_cards'] ?? 0,
      totalValue: (json['total_value'] as num?)?.toDouble() ?? 0.0,
      mtgCards: json['mtg_cards'] ?? 0,
      pokemonCards: json['pokemon_cards'] ?? 0,
      mtgValue: (json['mtg_value'] as num?)?.toDouble() ?? 0.0,
      pokemonValue: (json['pokemon_value'] as num?)?.toDouble() ?? 0.0,
    );
  }

  factory CollectionStats.empty() {
    return CollectionStats(
      totalCards: 0,
      uniqueCards: 0,
      totalValue: 0.0,
      mtgCards: 0,
      pokemonCards: 0,
      mtgValue: 0.0,
      pokemonValue: 0.0,
    );
  }

  String get displayTotalValue {
    return '\$${totalValue.toStringAsFixed(2)}';
  }

  String get displayMtgValue {
    return '\$${mtgValue.toStringAsFixed(2)}';
  }

  String get displayPokemonValue {
    return '\$${pokemonValue.toStringAsFixed(2)}';
  }
}
