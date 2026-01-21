import 'package:flutter/material.dart';
import '../models/card.dart';
import '../models/collection_item.dart' show PrintingType;
import '../models/mtg_grouped_result.dart';
import '../services/api_service.dart';
import '../utils/constants.dart';
import 'confirm_card_screen.dart';

class ScanResultScreen extends StatefulWidget {
  final List<CardModel> cards;
  final String searchQuery;
  final String game;
  final ScanMetadata? scanMetadata;
  final SetIconResult? setIcon;
  final ApiService? apiService;
  final List<int>? scannedImageBytes;
  final MTGGroupedResult? grouped; // For MTG 2-phase selection

  const ScanResultScreen({
    super.key,
    required this.cards,
    required this.searchQuery,
    required this.game,
    this.scanMetadata,
    this.setIcon,
    this.apiService,
    this.scannedImageBytes,
    this.grouped,
  });

  @override
  State<ScanResultScreen> createState() => _ScanResultScreenState();
}

class _ScanResultScreenState extends State<ScanResultScreen> {
  late final ApiService _apiService;
  int _quantity = 1;
  late String _condition;
  late PrintingType _printing;
  bool _isAdding = false;

  bool _isBrowsing = false;
  List<CardModel>? _browseResults;

  // MTG 2-phase selection state
  MTGSetGroup? _selectedSetGroup;

  // Use unified condition codes from constants
  List<String> get _conditions => CardConditions.codes;

  // Check if we should show MTG 2-phase UI
  bool get _showMTGGroupedUI =>
      widget.game == 'mtg' &&
      widget.grouped != null &&
      widget.grouped!.setGroups.isNotEmpty;

  @override
  void initState() {
    super.initState();
    _apiService = widget.apiService ?? ApiService();
    // Pre-fill printing type from scan metadata
    final meta = widget.scanMetadata;
    _printing = meta?.suggestedPrinting ?? PrintingType.normal;
    // Pre-fill condition based on image analysis suggested condition
    final suggested = meta?.suggestedCondition;
    _condition = (suggested != null && _conditions.contains(suggested))
        ? suggested
        : 'NM';
  }

  Future<void> _browseCandidateSets() async {
    if (_isBrowsing) return;

    final setIcon = widget.setIcon;
    if (setIcon == null) return;

    final meta = widget.scanMetadata;
    final q = meta?.cardName;
    if (q == null || q.trim().isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'No detected card name available to browse. Retake the photo.',
            ),
          ),
        );
      }
      return;
    }

    final setIDs = setIcon.candidates.isNotEmpty
        ? setIcon.candidates.map((c) => c.setId).toList()
        : (setIcon.bestSetId.isNotEmpty ? [setIcon.bestSetId] : <String>[]);

    if (setIDs.isEmpty) return;

    setState(() {
      _isBrowsing = true;
      _browseResults = null;
    });

    try {
      // Use detected card name + restrict to likely sets.
      final result = await _apiService.searchCards(
        q,
        widget.game,
        setIDs: setIDs,
      );

      if (!mounted) return;
      setState(() {
        _browseResults = result.cards;
      });

      await showModalBottomSheet(
        context: context,
        isScrollControlled: true,
        builder: (context) {
          final results = _browseResults ?? <CardModel>[];
          return SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Browse likely prints',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Showing ${results.length} result(s) filtered by set icon candidates.',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 12),
                  SizedBox(
                    height: MediaQuery.of(context).size.height * 0.6,
                    child: results.isEmpty
                        ? const Center(
                            child: Text('No results for candidate sets'),
                          )
                        : ListView.separated(
                            itemCount: results.length,
                            separatorBuilder: (_, index) =>
                                const Divider(height: 1),
                            itemBuilder: (context, idx) {
                              final card = results[idx];
                              return ListTile(
                                leading: card.imageUrl != null
                                    ? ClipRRect(
                                        borderRadius: BorderRadius.circular(4),
                                        child: SizedBox(
                                          width: 44,
                                          child: AspectRatio(
                                            aspectRatio: 2.5 / 3.5,
                                            child: Image.network(
                                              card.imageUrl!,
                                              fit: BoxFit.cover,
                                              errorBuilder:
                                                  (
                                                    context,
                                                    error,
                                                    stackTrace,
                                                  ) => const Icon(Icons.image),
                                            ),
                                          ),
                                        ),
                                      )
                                    : const Icon(Icons.image),
                                title: Text(card.name),
                                subtitle: Text(
                                  '${card.displaySet} • ${card.displayPrice}',
                                ),
                                onTap: () async {
                                  Navigator.pop(context);
                                  final chosen = await _confirmCard(card);
                                  if (chosen != null && mounted) {
                                    _showAddDialog(chosen);
                                  }
                                },
                              );
                            },
                          ),
                  ),
                ],
              ),
            ),
          );
        },
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('Browse failed: ${e.toString()}')));
    } finally {
      if (mounted) {
        setState(() => _isBrowsing = false);
      }
    }
  }

  Future<void> _addToCollection(CardModel card) async {
    setState(() => _isAdding = true);

    try {
      await _apiService.addToCollection(
        card.id,
        quantity: _quantity,
        condition: _condition,
        printing: _printing,
        scannedImageBytes: widget.scannedImageBytes,
      );

      if (!mounted) return;

      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Added ${card.name} to collection!'),
          backgroundColor: Colors.green,
        ),
      );

      Navigator.pop(context);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Error: ${e.toString()}'),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _isAdding = false);
      }
    }
  }

  Future<CardModel?> _confirmCard(CardModel card) async {
    final result = await Navigator.push<ConfirmCardResult?>(
      context,
      MaterialPageRoute(
        builder: (context) => ConfirmCardScreen(
          card: card,
          game: widget.game,
          initialQuery: (widget.scanMetadata?.cardName?.isNotEmpty ?? false)
              ? (widget.scanMetadata?.cardName ?? '')
              : card.name,
          apiService: _apiService,
          scanMetadata: widget.scanMetadata,
          setIcon: widget.setIcon,
          scannedImageBytes: widget.scannedImageBytes,
        ),
      ),
    );

    if (!mounted || result == null) return null;
    if (result.action == ConfirmCardAction.retake) {
      Navigator.pop(context);
      return null;
    }

    return result.card;
  }

  void _showAddDialog(CardModel card) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (context) => StatefulBuilder(
        builder: (context, setModalState) => Padding(
          padding: EdgeInsets.only(
            bottom: MediaQuery.of(context).viewInsets.bottom,
            left: 16,
            right: 16,
            top: 16,
          ),
          // Wrap in SingleChildScrollView to handle overflow on small screens
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  'Add ${card.name}',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 16),
                // Quantity
                Row(
                  children: [
                    const Text('Quantity:'),
                    const Spacer(),
                    IconButton(
                      icon: const Icon(Icons.remove),
                      onPressed: _quantity > 1
                          ? () => setModalState(() => _quantity--)
                          : null,
                    ),
                    Text('$_quantity', style: const TextStyle(fontSize: 18)),
                    IconButton(
                      icon: const Icon(Icons.add),
                      onPressed: () => setModalState(() => _quantity++),
                    ),
                  ],
                ),
                // Condition with auto-detect indicator
                Row(
                  children: [
                    const Text('Condition:'),
                    if (widget.scanMetadata?.suggestedCondition != null) ...[
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: _getConditionColor(_condition),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: const Text(
                          'Auto',
                          style: TextStyle(fontSize: 10, color: Colors.white),
                        ),
                      ),
                    ],
                    const SizedBox(width: 8),
                    Expanded(
                      child: DropdownButton<String>(
                        value: _condition,
                        isExpanded: true,
                        items: _conditions.map((c) {
                          return DropdownMenuItem(
                            value: c,
                            child: Text('$c - ${_getConditionDescription(c)}'),
                          );
                        }).toList(),
                        onChanged: (value) {
                          if (value != null) {
                            setModalState(() => _condition = value);
                          }
                        },
                      ),
                    ),
                  ],
                ),
                // Printing type dropdown
                Row(
                  children: [
                    const Text('Printing:'),
                    if (widget.scanMetadata?.suggestedPrinting != null &&
                        widget.scanMetadata!.suggestedPrinting !=
                            PrintingType.normal) ...[
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: Theme.of(
                            context,
                          ).colorScheme.tertiaryContainer,
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: const Text(
                          'Auto',
                          style: TextStyle(fontSize: 10, color: Colors.white),
                        ),
                      ),
                    ],
                    const SizedBox(width: 8),
                    Expanded(
                      child: DropdownButton<PrintingType>(
                        value: _printing,
                        isExpanded: true,
                        items: PrintingType.values.map((p) {
                          return DropdownMenuItem(
                            value: p,
                            child: Text(_getPrintingDisplayName(p)),
                          );
                        }).toList(),
                        onChanged: (value) {
                          if (value != null) {
                            setModalState(() => _printing = value);
                          }
                        },
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: _isAdding
                      ? null
                      : () {
                          Navigator.pop(context);
                          _addToCollection(card);
                        },
                  child: _isAdding
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Add to Collection'),
                ),
                const SizedBox(height: 16),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildScanInfoCard() {
    final meta = widget.scanMetadata;
    final setIcon = widget.setIcon;
    if ((meta == null || meta.confidence == 0) && setIcon == null) {
      return const SizedBox.shrink();
    }

    final scanConfidence = meta?.confidence ?? 0.0;

    return Card(
      margin: const EdgeInsets.all(8),
      color: Theme.of(context).colorScheme.primaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.document_scanner,
                  size: 20,
                  color: Theme.of(context).colorScheme.onPrimaryContainer,
                ),
                const SizedBox(width: 8),
                Text(
                  'Scan Detection',
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.onPrimaryContainer,
                  ),
                ),
                const Spacer(),
                if (meta != null)
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: _getConfidenceColor(context, scanConfidence),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(
                      '${(scanConfidence * 100).toInt()}% confidence',
                      style: TextStyle(
                        fontSize: 12,
                        color: Theme.of(context).colorScheme.onPrimary,
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 8),
            if (meta != null)
              Text(
                meta.detectionSummary,
                style: TextStyle(
                  color: Theme.of(context).colorScheme.onPrimaryContainer,
                ),
              ),
            if (setIcon != null) ...[
              if (meta != null) const SizedBox(height: 8),
              _buildSetIconIndicator(setIcon),
            ],
            // Match reason indicator (shows how set was identified)
            if (meta?.matchReason != null) ...[
              const SizedBox(height: 8),
              _buildMatchReasonIndicator(meta!),
            ],
            // Condition assessment display
            if (meta?.suggestedCondition != null) ...[
              const SizedBox(height: 8),
              _buildConditionIndicator(meta!),
            ],
            // Foil confidence display
            if (meta?.foilConfidence != null &&
                (meta?.foilConfidence ?? 0) > 0) ...[
              const SizedBox(height: 8),
              _buildFoilConfidenceIndicator(meta!),
            ],
            // Corner scores visualization
            if (meta?.cornerScores != null &&
                meta!.cornerScores!.isNotEmpty) ...[
              const SizedBox(height: 8),
              _buildCornerScoresGrid(meta.cornerScores!),
            ],
            if (meta?.foilIndicators.isNotEmpty ?? false) ...[
              const SizedBox(height: 4),
              Wrap(
                spacing: 4,
                children: meta!.foilIndicators.map((indicator) {
                  return Chip(
                    label: Text(
                      indicator,
                      style: const TextStyle(fontSize: 10),
                    ),
                    visualDensity: VisualDensity.compact,
                    backgroundColor: Colors.amber.shade100,
                    padding: EdgeInsets.zero,
                  );
                }).toList(),
              ),
            ],
            if (meta?.firstEdIndicators.isNotEmpty ?? false) ...[
              const SizedBox(height: 4),
              Wrap(
                spacing: 4,
                children: meta!.firstEdIndicators.map((indicator) {
                  return Chip(
                    label: Text(
                      indicator,
                      style: const TextStyle(fontSize: 10),
                    ),
                    visualDensity: VisualDensity.compact,
                    backgroundColor: Colors.amber.shade700,
                    labelStyle: const TextStyle(
                      color: Colors.white,
                      fontSize: 10,
                    ),
                    padding: EdgeInsets.zero,
                  );
                }).toList(),
              ),
            ],
            if (meta?.conditionHints.isNotEmpty ?? false) ...[
              const SizedBox(height: 4),
              Text(
                'Condition hints: ${meta!.conditionHints.join(", ")}',
                style: TextStyle(
                  fontSize: 12,
                  fontStyle: FontStyle.italic,
                  color: Theme.of(
                    context,
                  ).colorScheme.onPrimaryContainer.withValues(alpha: 0.7),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildMatchReasonIndicator(ScanMetadata meta) {
    final isAmbiguous = meta.isSetAmbiguous;
    final hasHighConfidence = meta.hasHighConfidenceSet;

    final bg = hasHighConfidence
        ? Theme.of(context).colorScheme.tertiaryContainer
        : (isAmbiguous ? Colors.amber.shade100 : Colors.grey.shade200);
    final fg = hasHighConfidence
        ? Theme.of(context).colorScheme.onTertiaryContainer
        : (isAmbiguous ? Colors.amber.shade900 : Colors.grey.shade700);

    return Row(
      children: [
        Icon(
          hasHighConfidence
              ? Icons.check_circle
              : (isAmbiguous ? Icons.help_outline : Icons.info_outline),
          size: 16,
          color: fg,
        ),
        const SizedBox(width: 4),
        Text(
          'Set match: ',
          style: TextStyle(
            fontSize: 12,
            color: Theme.of(context).colorScheme.onPrimaryContainer,
          ),
        ),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          decoration: BoxDecoration(
            color: bg,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            meta.matchReasonDescription,
            style: TextStyle(
              fontSize: 12,
              color: fg,
              fontWeight: FontWeight.bold,
            ),
          ),
        ),
        if (isAmbiguous) ...[
          const SizedBox(width: 8),
          Text(
            'Sets: ${meta.candidateSets.join(", ")}',
            style: TextStyle(
              fontSize: 11,
              color: Theme.of(
                context,
              ).colorScheme.onPrimaryContainer.withValues(alpha: 0.7),
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildSetIconIndicator(SetIconResult setIcon) {
    final confPct = (setIcon.confidence * 100).toInt();
    final label = setIcon.lowConfidence
        ? 'Set unsure ($confPct%)'
        : 'Set match ($confPct%)';

    final bg = setIcon.lowConfidence
        ? Colors.amber.shade100
        : Theme.of(context).colorScheme.tertiaryContainer;
    final fg = setIcon.lowConfidence
        ? Colors.amber.shade900
        : Theme.of(context).colorScheme.onTertiaryContainer;

    return Row(
      children: [
        Icon(
          Icons.badge,
          size: 16,
          color: setIcon.lowConfidence ? Colors.amber.shade900 : fg,
        ),
        const SizedBox(width: 4),
        Text(
          'Set icon: ',
          style: TextStyle(
            fontSize: 12,
            color: Theme.of(context).colorScheme.onPrimaryContainer,
          ),
        ),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          decoration: BoxDecoration(
            color: bg,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 12,
              color: fg,
              fontWeight: FontWeight.bold,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildConditionIndicator(ScanMetadata meta) {
    final condition = meta.suggestedCondition!;
    final color = _getConditionColor(condition);
    final description = _getConditionDescription(condition);

    return Row(
      children: [
        Icon(Icons.verified, size: 16, color: color),
        const SizedBox(width: 4),
        Text(
          'Suggested Condition: ',
          style: TextStyle(
            fontSize: 12,
            color: Theme.of(context).colorScheme.onPrimaryContainer,
          ),
        ),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          decoration: BoxDecoration(
            color: color,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            condition,
            style: const TextStyle(
              fontSize: 12,
              color: Colors.white,
              fontWeight: FontWeight.bold,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            description,
            style: TextStyle(
              fontSize: 11,
              color: Theme.of(
                context,
              ).colorScheme.onPrimaryContainer.withValues(alpha: 0.7),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildFoilConfidenceIndicator(ScanMetadata meta) {
    final confidence = meta.foilConfidence!;
    final isHighConfidence = confidence >= 0.7;

    return Row(
      children: [
        Icon(
          Icons.auto_awesome,
          size: 16,
          color: isHighConfidence ? Colors.amber : Colors.grey,
        ),
        const SizedBox(width: 4),
        Text(
          'Foil Detection: ',
          style: TextStyle(
            fontSize: 12,
            color: Theme.of(context).colorScheme.onPrimaryContainer,
          ),
        ),
        Container(
          width: 60,
          height: 8,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(4),
            color: Colors.grey.shade300,
          ),
          child: FractionallySizedBox(
            alignment: Alignment.centerLeft,
            widthFactor: confidence,
            child: Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(4),
                color: isHighConfidence ? Colors.amber : Colors.grey,
              ),
            ),
          ),
        ),
        const SizedBox(width: 4),
        Text(
          '${(confidence * 100).toInt()}%',
          style: TextStyle(
            fontSize: 11,
            color: Theme.of(context).colorScheme.onPrimaryContainer,
          ),
        ),
      ],
    );
  }

  Widget _buildCornerScoresGrid(Map<String, double> cornerScores) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Edge Whitening Detection:',
          style: TextStyle(
            fontSize: 12,
            color: Theme.of(context).colorScheme.onPrimaryContainer,
          ),
        ),
        const SizedBox(height: 4),
        SizedBox(
          width: 80,
          height: 80,
          child: CustomPaint(painter: CornerScoresPainter(cornerScores)),
        ),
      ],
    );
  }

  Color _getConditionColor(String condition) {
    switch (condition) {
      case 'M':
        return Colors.blue;
      case 'NM':
        return Colors.green;
      case 'LP':
        return Colors.lightGreen;
      case 'MP':
        return Colors.orange;
      case 'HP':
        return Colors.deepOrange;
      case 'D':
        return Colors.red;
      default:
        return Colors.grey;
    }
  }

  String _getConditionDescription(String condition) {
    return CardConditions.getLabel(condition);
  }

  String _getPrintingDisplayName(PrintingType printing) {
    switch (printing) {
      case PrintingType.normal:
        return 'Normal';
      case PrintingType.foil:
        return 'Foil / Holo';
      case PrintingType.firstEdition:
        return '1st Edition';
      case PrintingType.reverseHolofoil:
        return 'Reverse Holo';
      case PrintingType.unlimited:
        return 'Unlimited';
    }
  }

  Color _getConfidenceColor(BuildContext context, double confidence) {
    final colorScheme = Theme.of(context).colorScheme;
    if (confidence >= 0.7) return colorScheme.primary;
    if (confidence >= 0.4) return colorScheme.tertiary;
    return colorScheme.error;
  }

  @override
  Widget build(BuildContext context) {
    final setIcon = widget.setIcon;

    // MTG 2-phase selection UI
    if (_showMTGGroupedUI) {
      return _buildMTGGroupedUI(context);
    }

    return Scaffold(
      appBar: AppBar(
        title: Text('Results for "${widget.searchQuery}"'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
        actions: [
          if (setIcon != null && setIcon.lowConfidence)
            TextButton(
              onPressed: () {
                Navigator.pop(context);
              },
              child: const Text('Retake'),
            ),
        ],
      ),
      body: widget.cards.isEmpty
          ? const Center(child: Text('No cards found'))
          : Column(
              children: [
                _buildScanInfoCard(),
                if (setIcon != null && setIcon.lowConfidence)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
                    child: Card(
                      color: Colors.amber.shade50,
                      child: Padding(
                        padding: const EdgeInsets.all(12),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Row(
                              children: [
                                Icon(
                                  Icons.warning_amber_rounded,
                                  color: Colors.amber.shade900,
                                ),
                                const SizedBox(width: 8),
                                const Expanded(
                                  child: Text(
                                    'Set icon match is unsure. Browse prints from likely sets, or retake the photo.',
                                  ),
                                ),
                              ],
                            ),
                            const SizedBox(height: 12),
                            Row(
                              children: [
                                Expanded(
                                  child: FilledButton.icon(
                                    onPressed: _browseCandidateSets,
                                    icon: const Icon(Icons.view_list),
                                    label: const Text('Browse'),
                                  ),
                                ),
                                const SizedBox(width: 12),
                                Expanded(
                                  child: OutlinedButton.icon(
                                    onPressed: () {
                                      Navigator.pop(context);
                                    },
                                    icon: const Icon(Icons.camera_alt),
                                    label: const Text('Retake'),
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                Expanded(
                  child: ListView.builder(
                    itemCount: widget.cards.length,
                    itemBuilder: (context, index) {
                      final card = widget.cards[index];
                      final isBestMatch = index == 0 && widget.cards.length > 1;
                      return Card(
                        margin: const EdgeInsets.symmetric(
                          horizontal: 8,
                          vertical: 4,
                        ),
                        color: isBestMatch
                            ? Theme.of(context).colorScheme.primaryContainer
                                  .withValues(alpha: 0.5)
                            : null,
                        shape: isBestMatch
                            ? RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(12),
                                side: BorderSide(
                                  color: Theme.of(context).colorScheme.primary,
                                  width: 2,
                                ),
                              )
                            : null,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            if (isBestMatch)
                              Container(
                                width: double.infinity,
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 12,
                                  vertical: 6,
                                ),
                                decoration: BoxDecoration(
                                  color: Theme.of(context).colorScheme.primary,
                                  borderRadius: const BorderRadius.only(
                                    topLeft: Radius.circular(10),
                                    topRight: Radius.circular(10),
                                  ),
                                ),
                                child: Row(
                                  children: [
                                    Icon(
                                      Icons.star,
                                      size: 16,
                                      color: Theme.of(
                                        context,
                                      ).colorScheme.onPrimary,
                                    ),
                                    const SizedBox(width: 4),
                                    Text(
                                      'Best Match',
                                      style: TextStyle(
                                        color: Theme.of(
                                          context,
                                        ).colorScheme.onPrimary,
                                        fontWeight: FontWeight.bold,
                                        fontSize: 12,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ListTile(
                              leading: card.imageUrl != null
                                  ? ClipRRect(
                                      borderRadius: BorderRadius.circular(4),
                                      child: SizedBox(
                                        width:
                                            MediaQuery.of(context).size.width *
                                            0.12,
                                        child: AspectRatio(
                                          aspectRatio: 2.5 / 3.5,
                                          child: Image.network(
                                            card.imageUrl!,
                                            fit: BoxFit.cover,
                                            errorBuilder:
                                                (context, error, stackTrace) =>
                                                    const Icon(Icons.image),
                                          ),
                                        ),
                                      ),
                                    )
                                  : const Icon(Icons.image),
                              title: Text(
                                card.name,
                                style: isBestMatch
                                    ? const TextStyle(
                                        fontWeight: FontWeight.bold,
                                      )
                                    : null,
                              ),
                              subtitle: Text(
                                '${card.displaySet} • ${card.displayPrice}',
                              ),
                              trailing: IconButton(
                                icon: const Icon(Icons.add_circle),
                                color: Theme.of(context).colorScheme.primary,
                                onPressed: () async {
                                  final chosen = await _confirmCard(card);
                                  if (chosen != null && mounted) {
                                    _showAddDialog(chosen);
                                  }
                                },
                              ),
                              onTap: () async {
                                final chosen = await _confirmCard(card);
                                if (chosen != null && mounted) {
                                  _showAddDialog(chosen);
                                }
                              },
                            ),
                          ],
                        ),
                      );
                    },
                  ),
                ),
              ],
            ),
    );
  }

  // ============================================================
  // MTG 2-Phase Selection UI
  // ============================================================

  /// Build the MTG grouped selection UI (2-phase: set selection, then variant selection)
  Widget _buildMTGGroupedUI(BuildContext context) {
    final grouped = widget.grouped!;

    return Scaffold(
      appBar: AppBar(
        title: Text(
          _selectedSetGroup != null
              ? _selectedSetGroup!.setName
              : grouped.cardName.isNotEmpty
              ? grouped.cardName
              : 'Select Set',
        ),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
        leading: _selectedSetGroup != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: () => setState(() => _selectedSetGroup = null),
              )
            : null,
      ),
      body: _selectedSetGroup != null
          ? _buildMTGVariantSelection(context, _selectedSetGroup!)
          : _buildMTGSetSelection(context, grouped),
    );
  }

  /// Phase 1: Set selection list
  Widget _buildMTGSetSelection(BuildContext context, MTGGroupedResult grouped) {
    return Column(
      children: [
        // Info card showing what was detected
        _buildScanInfoCard(),

        // Header
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
          child: Row(
            children: [
              Icon(
                Icons.collections_bookmark,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                '${grouped.totalSets} sets found',
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ],
          ),
        ),

        // Set list
        Expanded(
          child: ListView.builder(
            itemCount: grouped.setGroups.length,
            itemBuilder: (context, index) {
              final setGroup = grouped.setGroups[index];
              return _buildSetGroupTile(context, setGroup);
            },
          ),
        ),
      ],
    );
  }

  /// Build a single set group tile for Phase 1
  Widget _buildSetGroupTile(BuildContext context, MTGSetGroup setGroup) {
    final isBestMatch = setGroup.isBestMatch;

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      color: isBestMatch
          ? Theme.of(
              context,
            ).colorScheme.primaryContainer.withValues(alpha: 0.5)
          : null,
      shape: isBestMatch
          ? RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
              side: BorderSide(
                color: Theme.of(context).colorScheme.primary,
                width: 2,
              ),
            )
          : null,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (isBestMatch)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.primary,
                borderRadius: const BorderRadius.only(
                  topLeft: Radius.circular(10),
                  topRight: Radius.circular(10),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.star,
                    size: 16,
                    color: Theme.of(context).colorScheme.onPrimary,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Best Match',
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.onPrimary,
                      fontWeight: FontWeight.bold,
                      fontSize: 12,
                    ),
                  ),
                ],
              ),
            ),
          ListTile(
            leading: _buildSetIcon(setGroup.setCode),
            title: Text(
              setGroup.setName,
              style: isBestMatch
                  ? const TextStyle(fontWeight: FontWeight.bold)
                  : null,
            ),
            subtitle: Text(
              <String?>[setGroup.variantCountLabel, setGroup.releaseYear]
                  .whereType<String>()
                  .where((v) => v.isNotEmpty)
                  .join(' • '),
            ),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => setState(() => _selectedSetGroup = setGroup),
          ),
        ],
      ),
    );
  }

  /// Phase 2: Variant selection within a set
  Widget _buildMTGVariantSelection(BuildContext context, MTGSetGroup setGroup) {
    return Column(
      children: [
        // Header with set name and variant count
        Container(
          padding: const EdgeInsets.all(16),
          color: Theme.of(context).colorScheme.surfaceContainerHighest,
          child: Row(
            children: [
              _buildSetIcon(setGroup.setCode),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      setGroup.setName,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    Text(
                      'Select a variant',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),

        // Variant list with images
        Expanded(
          child: ListView.builder(
            itemCount: setGroup.variants.length,
            itemBuilder: (context, index) {
              final card = setGroup.variants[index];
              return _buildVariantTile(context, card);
            },
          ),
        ),
      ],
    );
  }

  /// Build a single variant tile for Phase 2
  Widget _buildVariantTile(BuildContext context, CardModel card) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: ListTile(
        leading: card.imageUrl != null
            ? ClipRRect(
                borderRadius: BorderRadius.circular(4),
                child: SizedBox(
                  width: 50,
                  child: AspectRatio(
                    aspectRatio: 2.5 / 3.5,
                    child: Image.network(
                      card.imageUrl!,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) =>
                          const Icon(Icons.image),
                    ),
                  ),
                ),
              )
            : const Icon(Icons.image),
        title: Text(card.variantLabel),
        subtitle: Text(
          '${card.cardNumber != null ? "#${card.cardNumber} • " : ""}${card.displayPrice}',
        ),
        trailing: IconButton(
          icon: const Icon(Icons.add_circle),
          color: Theme.of(context).colorScheme.primary,
          onPressed: () => _onVariantSelected(card),
        ),
        onTap: () => _onVariantSelected(card),
      ),
    );
  }

  /// Handle variant selection in Phase 2
  Future<void> _onVariantSelected(CardModel card) async {
    final chosen = await _confirmCard(card);
    if (chosen != null && mounted) {
      _showAddDialog(chosen);
    }
  }

  /// Build a set icon (uses set code as placeholder)
  Widget _buildSetIcon(String setCode) {
    // Could use Scryfall set icon URL in the future:
    // https://svgs.scryfall.io/sets/${setCode.toLowerCase()}.svg
    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(4),
        color: Colors.grey.shade200,
      ),
      child: Center(
        child: Text(
          setCode.toUpperCase(),
          style: const TextStyle(fontSize: 10, fontWeight: FontWeight.bold),
        ),
      ),
    );
  }
}

/// Custom painter for visualizing corner whitening scores
class CornerScoresPainter extends CustomPainter {
  final Map<String, double> cornerScores;

  CornerScoresPainter(this.cornerScores);

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()..style = PaintingStyle.fill;
    final borderPaint = Paint()
      ..style = PaintingStyle.stroke
      ..color = Colors.grey
      ..strokeWidth = 1;

    // Draw card outline
    final cardRect = Rect.fromLTWH(0, 0, size.width, size.height);
    canvas.drawRect(cardRect, borderPaint);

    final cornerSize = size.width * 0.25;

    // Draw corners with color based on whitening score
    _drawCorner(canvas, paint, 0, 0, cornerSize, cornerScores['topLeft'] ?? 0);
    _drawCorner(
      canvas,
      paint,
      size.width - cornerSize,
      0,
      cornerSize,
      cornerScores['topRight'] ?? 0,
    );
    _drawCorner(
      canvas,
      paint,
      0,
      size.height - cornerSize,
      cornerSize,
      cornerScores['bottomLeft'] ?? 0,
    );
    _drawCorner(
      canvas,
      paint,
      size.width - cornerSize,
      size.height - cornerSize,
      cornerSize,
      cornerScores['bottomRight'] ?? 0,
    );
  }

  void _drawCorner(
    Canvas canvas,
    Paint paint,
    double x,
    double y,
    double size,
    double score,
  ) {
    // Green = good (low whitening), Red = bad (high whitening)
    paint.color = Color.lerp(Colors.green, Colors.red, score) ?? Colors.grey;
    canvas.drawRect(Rect.fromLTWH(x, y, size, size), paint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => true;
}
