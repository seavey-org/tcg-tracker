import 'package:flutter/material.dart';
import '../models/price_status.dart';

/// A simple stats display card used on the dashboard
class StatsCard extends StatelessWidget {
  final String title;
  final String value;
  final IconData? icon;
  final Color? iconColor;
  final Widget? trailing;

  const StatsCard({
    super.key,
    required this.title,
    required this.value,
    this.icon,
    this.iconColor,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                if (icon != null) ...[
                  Icon(icon, size: 20, color: iconColor ?? colorScheme.primary),
                  const SizedBox(width: 8),
                ],
                Expanded(
                  child: Text(
                    title,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
                if (trailing != null) trailing!,
              ],
            ),
            const SizedBox(height: 8),
            Text(
              value,
              style: theme.textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
                color: colorScheme.onSurface,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// A stats card showing a game breakdown (MTG vs Pokemon)
class GameBreakdownCard extends StatelessWidget {
  final int mtgCount;
  final int pokemonCount;
  final String mtgValue;
  final String pokemonValue;

  const GameBreakdownCard({
    super.key,
    required this.mtgCount,
    required this.pokemonCount,
    required this.mtgValue,
    required this.pokemonValue,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Game Breakdown',
              style: theme.textTheme.bodySmall?.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 12),
            _buildGameRow(
              context,
              'MTG',
              mtgCount,
              mtgValue,
              colorScheme.primary,
            ),
            const SizedBox(height: 8),
            _buildGameRow(
              context,
              'Pokemon',
              pokemonCount,
              pokemonValue,
              colorScheme.secondary,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildGameRow(
    BuildContext context,
    String game,
    int count,
    String value,
    Color color,
  ) {
    final theme = Theme.of(context);

    return Row(
      children: [
        Container(
          width: 12,
          height: 12,
          decoration: BoxDecoration(
            color: color,
            borderRadius: BorderRadius.circular(3),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(child: Text(game, style: theme.textTheme.bodyMedium)),
        Text(
          '$count cards',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(width: 16),
        Text(
          value,
          style: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
      ],
    );
  }
}

/// A card showing price API quota status
class PriceQuotaCard extends StatelessWidget {
  final int remaining;
  final int dailyLimit;
  final String resetTime;
  final bool loading;

  const PriceQuotaCard({
    super.key,
    required this.remaining,
    required this.dailyLimit,
    required this.resetTime,
    this.loading = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final percentage = dailyLimit > 0 ? remaining / dailyLimit : 0.0;

    Color progressColor;
    if (percentage > 0.5) {
      progressColor = colorScheme.tertiary;
    } else if (percentage > 0.2) {
      progressColor = colorScheme.primary;
    } else {
      progressColor = colorScheme.error;
    }

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.api_outlined, size: 20, color: colorScheme.primary),
                const SizedBox(width: 8),
                Text(
                  'Pokemon Price API',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
                const Spacer(),
                if (loading)
                  const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
              ],
            ),
            const SizedBox(height: 12),
            ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: LinearProgressIndicator(
                value: percentage,
                backgroundColor: colorScheme.surfaceContainerHighest,
                valueColor: AlwaysStoppedAnimation<Color>(progressColor),
                minHeight: 8,
              ),
            ),
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  '$remaining / $dailyLimit remaining',
                  style: theme.textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  'Resets in $resetTime',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// A warning card showing cards that can't receive price updates
class UnmatchedCardsWarning extends StatefulWidget {
  final List<UnmatchedCard> unmatchedCards;

  const UnmatchedCardsWarning({super.key, required this.unmatchedCards});

  @override
  State<UnmatchedCardsWarning> createState() => _UnmatchedCardsWarningState();
}

class _UnmatchedCardsWarningState extends State<UnmatchedCardsWarning> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    if (widget.unmatchedCards.isEmpty) {
      return const SizedBox.shrink();
    }

    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final count = widget.unmatchedCards.length;

    return Card(
      color: colorScheme.errorContainer,
      child: Theme(
        data: theme.copyWith(dividerColor: Colors.transparent),
        child: ExpansionTile(
          initiallyExpanded: _expanded,
          onExpansionChanged: (expanded) {
            setState(() => _expanded = expanded);
          },
          leading: Icon(Icons.warning_amber_rounded, color: colorScheme.error),
          title: Text(
            '$count card${count == 1 ? '' : 's'} cannot receive price updates',
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w600,
              color: colorScheme.onErrorContainer,
            ),
          ),
          subtitle: Text(
            'These cards could not be matched in the pricing database',
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onErrorContainer.withValues(alpha: 0.85),
            ),
          ),
          children: [
            const Divider(height: 1),
            ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: widget.unmatchedCards.length,
              separatorBuilder: (context, index) => Divider(
                height: 1,
                color: colorScheme.onErrorContainer.withValues(alpha: 0.2),
              ),
              itemBuilder: (context, index) {
                final card = widget.unmatchedCards[index];
                return ListTile(
                  dense: true,
                  title: Text(
                    card.name,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w500,
                      color: colorScheme.onErrorContainer,
                    ),
                  ),
                  subtitle: Text(
                    '${card.setName}${card.cardNumber.isNotEmpty ? ' #${card.cardNumber}' : ''}',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: colorScheme.onErrorContainer.withValues(
                        alpha: 0.9,
                      ),
                    ),
                  ),
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}
