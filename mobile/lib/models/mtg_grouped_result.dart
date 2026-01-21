import 'card.dart';

/// Represents a set containing variants of a scanned MTG card
/// Used for 2-phase MTG card selection (select set, then select variant)
class MTGSetGroup {
  final String setCode;
  final String setName;
  final String? releasedAt;
  final bool isBestMatch;
  final List<CardModel> variants;

  MTGSetGroup({
    required this.setCode,
    required this.setName,
    this.releasedAt,
    required this.isBestMatch,
    required this.variants,
  });

  factory MTGSetGroup.fromJson(Map<String, dynamic> json) {
    return MTGSetGroup(
      setCode: json['set_code'] ?? '',
      setName: json['set_name'] ?? '',
      releasedAt: json['released_at'],
      isBestMatch: json['is_best_match'] ?? false,
      variants:
          (json['variants'] as List<dynamic>?)
              ?.map((c) => CardModel.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  /// Human-readable variant count label
  String get variantCountLabel {
    final count = variants.length;
    return count == 1 ? '1 variant' : '$count variants';
  }

  /// Get release year from date string (e.g., "2022-02-18" -> "2022")
  String? get releaseYear {
    if (releasedAt == null || releasedAt!.isEmpty) return null;
    final parts = releasedAt!.split('-');
    return parts.isNotEmpty ? parts[0] : null;
  }
}

/// Grouped result for MTG card scans to enable 2-phase selection
/// Phase 1: User selects set from setGroups
/// Phase 2: User selects variant within the chosen set
class MTGGroupedResult {
  final String cardName;
  final List<MTGSetGroup> setGroups;
  final int totalSets;

  MTGGroupedResult({
    required this.cardName,
    required this.setGroups,
    required this.totalSets,
  });

  factory MTGGroupedResult.fromJson(Map<String, dynamic> json) {
    return MTGGroupedResult(
      cardName: json['card_name'] ?? '',
      setGroups:
          (json['set_groups'] as List<dynamic>?)
              ?.map((g) => MTGSetGroup.fromJson(g as Map<String, dynamic>))
              .toList() ??
          [],
      totalSets: json['total_sets'] ?? 0,
    );
  }

  /// Get the best match set group, if any
  MTGSetGroup? get bestMatch {
    for (final group in setGroups) {
      if (group.isBestMatch) return group;
    }
    return setGroups.isNotEmpty ? setGroups.first : null;
  }
}
