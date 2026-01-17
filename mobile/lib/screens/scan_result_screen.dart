import 'package:flutter/material.dart';
import '../models/card.dart';
import '../services/api_service.dart';

class ScanResultScreen extends StatefulWidget {
  final List<CardModel> cards;
  final String searchQuery;
  final ScanMetadata? scanMetadata;
  final ApiService? apiService;

  const ScanResultScreen({
    super.key,
    required this.cards,
    required this.searchQuery,
    this.scanMetadata,
    this.apiService,
  });

  @override
  State<ScanResultScreen> createState() => _ScanResultScreenState();
}

class _ScanResultScreenState extends State<ScanResultScreen> {
  late final ApiService _apiService;
  int _quantity = 1;
  String _condition = 'NM';
  late bool _foil;
  bool _isAdding = false;

  final List<String> _conditions = ['M', 'NM', 'EX', 'GD', 'LP', 'PL', 'PR'];

  @override
  void initState() {
    super.initState();
    _apiService = widget.apiService ?? ApiService();
    // Pre-fill foil based on scan detection
    _foil = widget.scanMetadata?.isFoil ?? false;
  }

  Future<void> _addToCollection(CardModel card) async {
    setState(() => _isAdding = true);

    try {
      await _apiService.addToCollection(
        card.id,
        quantity: _quantity,
        condition: _condition,
        foil: _foil,
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
                // Condition
                Row(
                  children: [
                    const Text('Condition:'),
                    const SizedBox(width: 16),
                    Expanded(
                      child: DropdownButton<String>(
                        value: _condition,
                        isExpanded: true,
                        items: _conditions.map((c) {
                          return DropdownMenuItem(value: c, child: Text(c));
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
                // Foil with detection indicator
                SwitchListTile(
                  title: Row(
                    children: [
                      const Text('Foil'),
                      if (widget.scanMetadata?.isFoil == true) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: Theme.of(context).colorScheme.tertiaryContainer,
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Text(
                            'Detected',
                            style: TextStyle(
                              fontSize: 12,
                              color: Theme.of(context).colorScheme.onTertiaryContainer,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  value: _foil,
                  onChanged: (value) => setModalState(() => _foil = value),
                ),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: _isAdding ? null : () {
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
    if (meta == null || meta.confidence == 0) {
      return const SizedBox.shrink();
    }

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
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: _getConfidenceColor(context, meta.confidence),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    '${(meta.confidence * 100).toInt()}% confidence',
                    style: TextStyle(
                      fontSize: 12,
                      color: Theme.of(context).colorScheme.onPrimary,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              meta.detectionSummary,
              style: TextStyle(
                color: Theme.of(context).colorScheme.onPrimaryContainer,
              ),
            ),
            if (meta.foilIndicators.isNotEmpty) ...[
              const SizedBox(height: 4),
              Wrap(
                spacing: 4,
                children: meta.foilIndicators.map((indicator) {
                  return Chip(
                    label: Text(indicator, style: const TextStyle(fontSize: 10)),
                    visualDensity: VisualDensity.compact,
                    backgroundColor: Colors.amber.shade100,
                    padding: EdgeInsets.zero,
                  );
                }).toList(),
              ),
            ],
            if (meta.conditionHints.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                'Condition hints: ${meta.conditionHints.join(", ")}',
                style: TextStyle(
                  fontSize: 12,
                  fontStyle: FontStyle.italic,
                  color: Theme.of(context).colorScheme.onPrimaryContainer.withValues(alpha: 0.7),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Color _getConfidenceColor(BuildContext context, double confidence) {
    final colorScheme = Theme.of(context).colorScheme;
    if (confidence >= 0.7) return colorScheme.primary;
    if (confidence >= 0.4) return colorScheme.tertiary;
    return colorScheme.error;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Results for "${widget.searchQuery}"'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
      ),
      body: widget.cards.isEmpty
          ? const Center(child: Text('No cards found'))
          : Column(
              children: [
                _buildScanInfoCard(),
                Expanded(
                  child: ListView.builder(
                    itemCount: widget.cards.length,
                    itemBuilder: (context, index) {
                      final card = widget.cards[index];
                      return Card(
                        margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                        child: ListTile(
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
                                        errorBuilder: (context, error, stackTrace) => const Icon(Icons.image),
                                      ),
                                    ),
                                  ),
                                )
                              : const Icon(Icons.image),
                          title: Text(card.name),
                          subtitle: Text('${card.displaySet} • ${card.displayPrice}'),
                          trailing: IconButton(
                            icon: const Icon(Icons.add_circle),
                            color: Theme.of(context).colorScheme.primary,
                            onPressed: () => _showAddDialog(card),
                          ),
                          onTap: () => _showAddDialog(card),
                        ),
                      );
                    },
                  ),
                ),
              ],
            ),
    );
  }
}
