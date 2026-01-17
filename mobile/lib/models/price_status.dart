class PriceStatus {
  final int remaining;
  final int dailyLimit;
  final int requestsToday;
  final DateTime resetsAt;

  PriceStatus({
    required this.remaining,
    required this.dailyLimit,
    required this.requestsToday,
    required this.resetsAt,
  });

  factory PriceStatus.fromJson(Map<String, dynamic> json) {
    return PriceStatus(
      remaining: json['remaining'] ?? 0,
      dailyLimit: json['daily_limit'] ?? 100,
      requestsToday: json['requests_today'] ?? 0,
      resetsAt: json['resets_at'] != null
          ? DateTime.parse(json['resets_at'])
          : DateTime.now().add(const Duration(days: 1)),
    );
  }

  factory PriceStatus.empty() {
    return PriceStatus(
      remaining: 100,
      dailyLimit: 100,
      requestsToday: 0,
      resetsAt: DateTime.now().add(const Duration(days: 1)),
    );
  }

  /// Percentage of quota remaining (0.0 to 1.0)
  double get remainingPercent {
    if (dailyLimit == 0) return 0.0;
    return remaining / dailyLimit;
  }

  /// Percentage of quota used (0.0 to 1.0)
  double get usedPercent {
    if (dailyLimit == 0) return 0.0;
    return requestsToday / dailyLimit;
  }

  /// Human-readable time until reset
  String get resetTimeDisplay {
    final now = DateTime.now();
    final diff = resetsAt.difference(now);

    if (diff.isNegative) return 'Soon';
    if (diff.inHours > 0) {
      return '${diff.inHours}h ${diff.inMinutes % 60}m';
    }
    if (diff.inMinutes > 0) {
      return '${diff.inMinutes}m';
    }
    return 'Soon';
  }

  /// Whether we have quota remaining
  bool get hasQuota => remaining > 0;

  /// Whether quota is low (< 20%)
  bool get isLow => remainingPercent < 0.2;

  /// Whether quota is critical (< 5%)
  bool get isCritical => remainingPercent < 0.05;
}
