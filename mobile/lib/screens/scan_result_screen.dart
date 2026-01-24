import 'package:flutter/material.dart';
import '../models/card.dart';
import '../models/collection_item.dart' show PrintingType;
import '../models/gemini_scan_result.dart';
import '../services/api_service.dart';
import '../utils/constants.dart';
import 'confirm_card_screen.dart';

/// Screen displaying Gemini card identification results
/// Supports both single-card selection and MTG 2-phase (set -> variant) selection
class ScanResultScreen extends StatefulWidget {
  final GeminiScanResult geminiResult;
  final ApiService? apiService;
  final List<int>? scannedImageBytes;

  const ScanResultScreen({
    super.key,
    required this.geminiResult,
    this.apiService,
    this.scannedImageBytes,
  });

  @override
  State<ScanResultScreen> createState() => _ScanResultScreenState();
}

class _ScanResultScreenState extends State<ScanResultScreen> {
  late final ApiService _apiService;
  int _quantity = 1;
  late String _condition;
  late PrintingType _printing;
  late String _language; // Language for collection (defaults to observed)
  bool _isAdding = false;

  // MTG 2-phase selection state
  MTGSetInfo? _selectedSet;

  // Condition codes from constants
  List<String> get _conditions => CardConditions.codes;

  // Available languages for collection
  static const _availableLanguages = [
    'English',
    'Japanese',
    'German',
    'French',
    'Italian',
    'Spanish',
    'Korean',
    'Portuguese',
    'Chinese',
  ];

  // Check if we should show MTG 2-phase UI
  bool get _showMTGGroupedUI {
    final result = widget.geminiResult;
    return result.game == 'mtg' && result.cards.length > 1;
  }

  @override
  void initState() {
    super.initState();
    _apiService = widget.apiService ?? ApiService();
    _condition = 'NM';
    _printing = PrintingType.normal;
    // Default language to what Gemini observed
    _language = widget.geminiResult.observedLanguage.isNotEmpty
        ? widget.geminiResult.observedLanguage
        : 'English';
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
        language: _language,
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
          game: widget.geminiResult.game,
          initialQuery: widget.geminiResult.canonicalNameEN.isNotEmpty
              ? widget.geminiResult.canonicalNameEN
              : card.name,
          apiService: _apiService,
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
                // Condition
                Row(
                  children: [
                    const Text('Condition:'),
                    const SizedBox(width: 8),
                    Expanded(
                      child: DropdownButton<String>(
                        value: _condition,
                        isExpanded: true,
                        items: _conditions.map((c) {
                          return DropdownMenuItem(
                            value: c,
                            child: Text('$c - ${CardConditions.getLabel(c)}'),
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
                // Printing type
                Row(
                  children: [
                    const Text('Printing:'),
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
                // Language selector
                Row(
                  children: [
                    const Text('Language:'),
                    const SizedBox(width: 8),
                    if (widget.geminiResult.isNonEnglish)
                      Container(
                        margin: const EdgeInsets.only(right: 8),
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
                          'Detected',
                          style: TextStyle(fontSize: 10),
                        ),
                      ),
                    Expanded(
                      child: DropdownButton<String>(
                        value: _availableLanguages.contains(_language)
                            ? _language
                            : 'English',
                        isExpanded: true,
                        items: _availableLanguages.map((lang) {
                          return DropdownMenuItem(
                            value: lang,
                            child: Row(
                              children: [
                                Text(_getLanguageFlag(lang)),
                                const SizedBox(width: 8),
                                Text(lang),
                              ],
                            ),
                          );
                        }).toList(),
                        onChanged: (value) {
                          if (value != null) {
                            setModalState(() => _language = value);
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

  String _getLanguageFlag(String language) {
    switch (language.toLowerCase()) {
      case 'english':
        return '\u{1F1FA}\u{1F1F8}'; // US flag
      case 'japanese':
        return '\u{1F1EF}\u{1F1F5}'; // Japan flag
      case 'german':
        return '\u{1F1E9}\u{1F1EA}'; // Germany flag
      case 'french':
        return '\u{1F1EB}\u{1F1F7}'; // France flag
      case 'italian':
        return '\u{1F1EE}\u{1F1F9}'; // Italy flag
      case 'spanish':
        return '\u{1F1EA}\u{1F1F8}'; // Spain flag
      case 'korean':
        return '\u{1F1F0}\u{1F1F7}'; // Korea flag
      case 'portuguese':
        return '\u{1F1F5}\u{1F1F9}'; // Portugal flag
      case 'chinese':
        return '\u{1F1E8}\u{1F1F3}'; // China flag
      default:
        return '\u{1F30D}'; // Globe
    }
  }

  Widget _buildGeminiInfoCard() {
    final result = widget.geminiResult;

    return Card(
      margin: const EdgeInsets.all(8),
      color: Theme.of(context).colorScheme.primaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header row with Gemini icon and confidence
            Row(
              children: [
                Icon(
                  Icons.auto_awesome,
                  size: 20,
                  color: Theme.of(context).colorScheme.onPrimaryContainer,
                ),
                const SizedBox(width: 8),
                Text(
                  'Gemini Identification',
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.onPrimaryContainer,
                  ),
                ),
                const Spacer(),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: _getConfidenceColor(result.confidence),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    '${(result.confidence * 100).toInt()}% ${result.confidenceLabel}',
                    style: TextStyle(
                      fontSize: 12,
                      color: Theme.of(context).colorScheme.onPrimary,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            // Detected info
            Text(
              _buildDetectionSummary(result),
              style: TextStyle(
                color: Theme.of(context).colorScheme.onPrimaryContainer,
              ),
            ),
            // Language badge if non-English
            if (result.isNonEnglish) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Icon(
                    Icons.language,
                    size: 16,
                    color: Theme.of(context).colorScheme.onPrimaryContainer,
                  ),
                  const SizedBox(width: 4),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: Theme.of(context).colorScheme.tertiaryContainer,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          _getLanguageFlag(result.observedLanguage),
                          style: const TextStyle(fontSize: 14),
                        ),
                        const SizedBox(width: 4),
                        Text(
                          result.observedLanguage,
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.bold,
                            color: Theme.of(
                              context,
                            ).colorScheme.onTertiaryContainer,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ],
            // Reasoning (collapsed by default)
            if (result.reasoning.isNotEmpty) ...[
              const SizedBox(height: 8),
              ExpansionTile(
                title: Text(
                  'How it was identified',
                  style: TextStyle(
                    fontSize: 12,
                    color: Theme.of(
                      context,
                    ).colorScheme.onPrimaryContainer.withValues(alpha: 0.7),
                  ),
                ),
                tilePadding: EdgeInsets.zero,
                childrenPadding: const EdgeInsets.only(bottom: 8),
                children: [
                  Text(
                    result.reasoning,
                    style: TextStyle(
                      fontSize: 12,
                      fontStyle: FontStyle.italic,
                      color: Theme.of(context).colorScheme.onPrimaryContainer,
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }

  String _buildDetectionSummary(GeminiScanResult result) {
    final parts = <String>[];
    if (result.canonicalNameEN.isNotEmpty) {
      parts.add(result.canonicalNameEN);
    } else if (result.cardName.isNotEmpty) {
      parts.add(result.cardName);
    }
    if (result.setName.isNotEmpty) {
      parts.add(result.setName);
    } else if (result.setCode.isNotEmpty) {
      parts.add(result.setCode);
    }
    if (result.cardNumber.isNotEmpty) {
      parts.add('#${result.cardNumber}');
    }
    return parts.isEmpty ? 'Card detected' : parts.join(' - ');
  }

  Color _getConfidenceColor(double confidence) {
    final colorScheme = Theme.of(context).colorScheme;
    if (confidence >= 0.7) return colorScheme.primary;
    if (confidence >= 0.4) return colorScheme.tertiary;
    return colorScheme.error;
  }

  @override
  Widget build(BuildContext context) {
    // MTG 2-phase selection UI
    if (_showMTGGroupedUI) {
      return _buildMTGGroupedUI(context);
    }

    // Standard single-game or Pokemon flow
    return _buildStandardUI(context);
  }

  Widget _buildStandardUI(BuildContext context) {
    final result = widget.geminiResult;
    final cards = result.cards;
    final displayName = result.canonicalNameEN.isNotEmpty
        ? result.canonicalNameEN
        : (result.cardName.isNotEmpty ? result.cardName : 'Scanned Card');

    return Scaffold(
      appBar: AppBar(
        title: Text('Results for "$displayName"'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
        actions: [
          if (result.confidence < 0.5)
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('Retake'),
            ),
        ],
      ),
      body: cards.isEmpty
          ? const Center(child: Text('No cards found'))
          : Column(
              children: [
                _buildGeminiInfoCard(),
                Expanded(
                  child: ListView.builder(
                    itemCount: cards.length,
                    itemBuilder: (context, index) {
                      final card = cards[index];
                      final isBestMatch = index == 0 && cards.length > 1;
                      return _buildCardTile(card, isBestMatch: isBestMatch);
                    },
                  ),
                ),
              ],
            ),
    );
  }

  Widget _buildCardTile(CardModel card, {bool isBestMatch = false}) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
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
            leading: card.imageUrl != null
                ? ClipRRect(
                    borderRadius: BorderRadius.circular(4),
                    child: SizedBox(
                      width: MediaQuery.of(context).size.width * 0.12,
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
            title: Text(
              card.name,
              style: isBestMatch
                  ? const TextStyle(fontWeight: FontWeight.bold)
                  : null,
            ),
            subtitle: Text('${card.displaySet} - ${card.displayPrice}'),
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
  }

  // ============================================================
  // MTG 2-Phase Selection UI
  // ============================================================

  Widget _buildMTGGroupedUI(BuildContext context) {
    final result = widget.geminiResult;

    return Scaffold(
      appBar: AppBar(
        title: Text(
          _selectedSet != null
              ? _selectedSet!.setName
              : result.canonicalNameEN.isNotEmpty
              ? result.canonicalNameEN
              : 'Select Set',
        ),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
        leading: _selectedSet != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: () => setState(() => _selectedSet = null),
              )
            : null,
      ),
      body: _selectedSet != null
          ? _buildMTGVariantSelection(context, _selectedSet!)
          : _buildMTGSetSelection(context, result),
    );
  }

  /// Phase 1: Set selection list
  Widget _buildMTGSetSelection(BuildContext context, GeminiScanResult result) {
    final sets = result.getMTGSets();

    return Column(
      children: [
        // Info card showing Gemini detection
        _buildGeminiInfoCard(),

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
                '${sets.length} sets found',
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ],
          ),
        ),

        // Set list
        Expanded(
          child: ListView.builder(
            itemCount: sets.length,
            itemBuilder: (context, index) {
              final setInfo = sets[index];
              return _buildSetGroupTile(context, setInfo);
            },
          ),
        ),
      ],
    );
  }

  /// Build a single set group tile for Phase 1
  Widget _buildSetGroupTile(BuildContext context, MTGSetInfo setInfo) {
    final isBestMatch = setInfo.isBestMatch;

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
            leading: _buildSetIcon(setInfo.setCode),
            title: Text(
              setInfo.setName,
              style: isBestMatch
                  ? const TextStyle(fontWeight: FontWeight.bold)
                  : null,
            ),
            subtitle: Text(
              <String?>[
                setInfo.variantCountLabel,
                setInfo.releaseYear,
              ].whereType<String>().where((v) => v.isNotEmpty).join(' - '),
            ),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => setState(() => _selectedSet = setInfo),
          ),
        ],
      ),
    );
  }

  /// Phase 2: Variant selection within a set
  Widget _buildMTGVariantSelection(BuildContext context, MTGSetInfo setInfo) {
    final grouped = widget.geminiResult.groupCardsBySet();
    final variants = grouped[setInfo.setCode] ?? [];

    return Column(
      children: [
        // Header with set name and variant count
        Container(
          padding: const EdgeInsets.all(16),
          color: Theme.of(context).colorScheme.surfaceContainerHighest,
          child: Row(
            children: [
              _buildSetIcon(setInfo.setCode),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      setInfo.setName,
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
            itemCount: variants.length,
            itemBuilder: (context, index) {
              final card = variants[index];
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
          '${card.cardNumber != null ? "#${card.cardNumber} - " : ""}${card.displayPrice}',
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
