import 'dart:typed_data';

import 'package:flutter/material.dart';
import '../models/card.dart';
import '../services/api_service.dart';
import '../utils/constants.dart';

enum ConfirmCardAction { confirmed, retake }

class ConfirmCardResult {
  final ConfirmCardAction action;
  final CardModel? card;

  const ConfirmCardResult._(this.action, this.card);

  const ConfirmCardResult.confirmed(CardModel card)
    : this._(ConfirmCardAction.confirmed, card);

  const ConfirmCardResult.retake() : this._(ConfirmCardAction.retake, null);
}

class ConfirmCardScreen extends StatefulWidget {
  final CardModel card;
  final String game;
  final String initialQuery;
  final ApiService? apiService;
  final List<int>? scannedImageBytes;

  const ConfirmCardScreen({
    super.key,
    required this.card,
    required this.game,
    required this.initialQuery,
    this.apiService,
    this.scannedImageBytes,
  });

  @override
  State<ConfirmCardScreen> createState() => _ConfirmCardScreenState();
}

class _ConfirmCardScreenState extends State<ConfirmCardScreen> {
  late final ApiService _apiService;
  final TextEditingController _searchController = TextEditingController();

  bool _isSearching = false;
  String? _searchError;
  List<CardModel>? _searchResults;

  // Toggle between scanned photo and official image
  bool _showScannedImage = true;

  bool get _hasScannedImage =>
      widget.scannedImageBytes != null && widget.scannedImageBytes!.isNotEmpty;

  @override
  void initState() {
    super.initState();
    _apiService = widget.apiService ?? ApiService();
    _searchController.text = widget.initialQuery;
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _search() async {
    final q = _searchController.text.trim();
    if (q.isEmpty || _isSearching) return;

    setState(() {
      _isSearching = true;
      _searchError = null;
      _searchResults = null;
    });

    try {
      final result = await _apiService.searchCards(q, widget.game);

      if (!mounted) return;
      setState(() {
        _searchResults = result.cards;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _searchError = e.toString();
      });
    } finally {
      if (mounted) setState(() => _isSearching = false);
    }
  }

  void _pickCard(CardModel card) {
    Navigator.pop(context, ConfirmCardResult.confirmed(card));
  }

  void _retake() {
    Navigator.pop(context, const ConfirmCardResult.retake());
  }

  Widget _buildCardImage() {
    if (_hasScannedImage && _showScannedImage) {
      return Image.memory(
        Uint8List.fromList(widget.scannedImageBytes!),
        key: const ValueKey('scanned'),
        fit: BoxFit.cover,
        errorBuilder: (context, error, stackTrace) => const Icon(Icons.image),
      );
    }

    if (widget.card.imageUrl != null) {
      return Image.network(
        widget.card.imageUrl!,
        key: const ValueKey('official'),
        fit: BoxFit.cover,
        errorBuilder: (context, error, stackTrace) => const Icon(Icons.image),
      );
    }

    return const Icon(Icons.image, key: ValueKey('placeholder'));
  }

  Widget _buildTogglePill(String label, bool isActive) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: isActive
            ? theme.colorScheme.primary
            : theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 10,
          fontWeight: isActive ? FontWeight.bold : FontWeight.normal,
          color: isActive
              ? theme.colorScheme.onPrimary
              : theme.colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Confirm Card'),
        actions: [TextButton(onPressed: _retake, child: const Text('Retake'))],
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Image section with toggle support
                  Column(
                    children: [
                      GestureDetector(
                        onTap: _hasScannedImage
                            ? () => setState(
                                () => _showScannedImage = !_showScannedImage,
                              )
                            : null,
                        child: SizedBox(
                          width: 120,
                          child: AspectRatio(
                            aspectRatio: 2.5 / 3.5,
                            child: Card(
                              clipBehavior: Clip.antiAlias,
                              child: AnimatedSwitcher(
                                duration: const Duration(milliseconds: 200),
                                child: _buildCardImage(),
                              ),
                            ),
                          ),
                        ),
                      ),
                      // Toggle indicator (only if scanned image available)
                      if (_hasScannedImage) ...[
                        const SizedBox(height: 8),
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            _buildTogglePill('Your Photo', _showScannedImage),
                            const SizedBox(width: 4),
                            _buildTogglePill('Official', !_showScannedImage),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'Tap image to compare',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                            fontSize: 10,
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          widget.card.name,
                          style: theme.textTheme.titleLarge?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 8),
                        Text(widget.card.displaySet),
                        if (widget.card.cardNumber != null)
                          Text('No. ${widget.card.cardNumber}'),
                        const SizedBox(height: 8),
                        Text(
                          'Price: ${widget.card.displayPrice}',
                          style: theme.textTheme.bodyMedium,
                        ),
                      ],
                    ),
                  ),
                ],
              ),

              const SizedBox(height: 16),
              Card(
                color: theme.colorScheme.surfaceContainerHighest,
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        'Before adding, confirm this matches your physical card (set + number + artwork).',
                        style: theme.textTheme.bodyMedium,
                      ),
                      const SizedBox(height: 12),
                      FilledButton.icon(
                        onPressed: () => Navigator.pop(
                          context,
                          ConfirmCardResult.confirmed(widget.card),
                        ),
                        icon: const Icon(Icons.check),
                        label: const Text('Yes, this is correct'),
                      ),
                      const SizedBox(height: 8),
                      OutlinedButton.icon(
                        onPressed: _retake,
                        icon: const Icon(Icons.camera_alt),
                        label: const Text('Retake photo'),
                      ),
                    ],
                  ),
                ),
              ),

              const SizedBox(height: 16),
              Text('Search manually', style: theme.textTheme.titleMedium),
              const SizedBox(height: 8),
              TextField(
                controller: _searchController,
                decoration: const InputDecoration(
                  prefixIcon: Icon(Icons.search),
                  hintText: 'Search card name...',
                  border: OutlineInputBorder(),
                ),
                textInputAction: TextInputAction.search,
                onSubmitted: (_) => _search(),
              ),
              const SizedBox(height: 8),
              FilledButton(
                onPressed: _isSearching ? null : _search,
                child: _isSearching
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Search'),
              ),
              if (_searchError != null) ...[
                const SizedBox(height: 8),
                Text(
                  _searchError!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
              ],
              if (_searchResults != null) ...[
                const SizedBox(height: 12),
                SizedBox(
                  height: MediaQuery.of(context).size.height * 0.45,
                  child: _searchResults!.isEmpty
                      ? const Center(child: Text('No results'))
                      : ListView.separated(
                          itemCount: _searchResults!.length,
                          separatorBuilder: (_, index) =>
                              const Divider(height: 1),
                          itemBuilder: (context, idx) {
                            final card = _searchResults![idx];
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
                                                (context, error, stackTrace) =>
                                                    const Icon(Icons.image),
                                          ),
                                        ),
                                      ),
                                    )
                                  : const Icon(Icons.image),
                              title: Text(card.name),
                              subtitle: Text(
                                '${card.displaySet} • ${card.displayPrice}',
                              ),
                              trailing: const Icon(Icons.chevron_right),
                              onTap: () => _pickCard(card),
                            );
                          },
                        ),
                ),
              ],

              const SizedBox(height: 12),
              Text(
                'Condition shortcuts: ${CardConditions.codes.join(" / ")}',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
