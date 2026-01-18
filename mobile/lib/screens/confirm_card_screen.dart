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
  final ScanMetadata? scanMetadata;
  final SetIconResult? setIcon;
  final List<int>? scannedImageBytes;

  const ConfirmCardScreen({
    super.key,
    required this.card,
    required this.game,
    required this.initialQuery,
    this.apiService,
    this.scanMetadata,
    this.setIcon,
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

  bool _isBrowsing = false;
  String? _browseError;
  List<CardModel>? _browseResults;

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

  Future<void> _browseLikelyPrints() async {
    if (_isBrowsing) return;

    final setIcon = widget.setIcon;
    if (setIcon == null) return;

    final q = widget.scanMetadata?.cardName?.trim();
    if (q == null || q.isEmpty) {
      if (!mounted) return;
      setState(() {
        _browseError =
            'No detected card name available to browse. Retake the photo.';
        _browseResults = null;
      });
      return;
    }

    final setIDs = setIcon.candidates.isNotEmpty
        ? setIcon.candidates.map((c) => c.setId).toList()
        : (setIcon.bestSetId != null ? [setIcon.bestSetId!] : <String>[]);

    if (setIDs.isEmpty) return;

    setState(() {
      _isBrowsing = true;
      _browseError = null;
      _browseResults = null;
    });

    try {
      final result = await _apiService.searchCards(
        q,
        widget.game,
        setIDs: setIDs,
      );

      if (!mounted) return;
      setState(() {
        _browseResults = result.cards;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _browseError = e.toString();
      });
    } finally {
      if (mounted) setState(() => _isBrowsing = false);
    }
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
      final setIcon = widget.setIcon;
      final setIDs = (setIcon != null)
          ? (setIcon.candidates.isNotEmpty
                ? setIcon.candidates.map((c) => c.setId).toList()
                : (setIcon.bestSetId != null
                      ? [setIcon.bestSetId!]
                      : <String>[]))
          : <String>[];

      final result = await _apiService.searchCards(
        q,
        widget.game,
        setIDs: setIDs.isNotEmpty ? setIDs : null,
      );

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

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final setIcon = widget.setIcon;
    final confPct = setIcon == null ? null : (setIcon.confidence * 100).toInt();

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
                  SizedBox(
                    width: 120,
                    child: AspectRatio(
                      aspectRatio: 2.5 / 3.5,
                      child: Card(
                        clipBehavior: Clip.antiAlias,
                        child: widget.card.imageUrl != null
                            ? Image.network(
                                widget.card.imageUrl!,
                                fit: BoxFit.cover,
                                errorBuilder: (context, error, stackTrace) =>
                                    const Icon(Icons.image),
                              )
                            : const Icon(Icons.image),
                      ),
                    ),
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
                        if (setIcon != null) ...[
                          const SizedBox(height: 8),
                          Text(
                            setIcon.lowConfidence
                                ? 'Set icon unsure ($confPct%)'
                                : 'Set icon match ($confPct%)',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: setIcon.lowConfidence
                                  ? Colors.amber.shade900
                                  : theme.colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),

              const SizedBox(height: 16),
              if (setIcon != null && setIcon.lowConfidence)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
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
                              Expanded(
                                child: Text(
                                  'Set icon match is unsure. Consider retaking the photo, browsing likely prints, or using manual search to choose the correct printing.',
                                  style: theme.textTheme.bodyMedium,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              Expanded(
                                child: FilledButton.icon(
                                  onPressed: _isBrowsing
                                      ? null
                                      : _browseLikelyPrints,
                                  icon: const Icon(Icons.view_list),
                                  label: const Text('Browse likely prints'),
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: OutlinedButton.icon(
                                  onPressed: _retake,
                                  icon: const Icon(Icons.camera_alt),
                                  label: const Text('Retake photo'),
                                ),
                              ),
                            ],
                          ),
                          if (_browseError != null) ...[
                            const SizedBox(height: 8),
                            Text(
                              _browseError!,
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: theme.colorScheme.error,
                              ),
                            ),
                          ],
                          if (_browseResults != null) ...[
                            const SizedBox(height: 12),
                            Text(
                              'Likely prints',
                              style: theme.textTheme.titleSmall?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                            const SizedBox(height: 8),
                            SizedBox(
                              height: MediaQuery.of(context).size.height * 0.35,
                              child: _browseResults!.isEmpty
                                  ? const Center(
                                      child: Text(
                                        'No results for candidate sets',
                                      ),
                                    )
                                  : ListView.separated(
                                      itemCount: _browseResults!.length,
                                      separatorBuilder: (_, __) =>
                                          const Divider(height: 1),
                                      itemBuilder: (context, idx) {
                                        final card = _browseResults![idx];
                                        return ListTile(
                                          leading: card.imageUrl != null
                                              ? ClipRRect(
                                                  borderRadius:
                                                      BorderRadius.circular(4),
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
                                                            ) => const Icon(
                                                              Icons.image,
                                                            ),
                                                      ),
                                                    ),
                                                  ),
                                                )
                                              : const Icon(Icons.image),
                                          title: Text(card.name),
                                          subtitle: Text(
                                            '${card.displaySet} • ${card.displayPrice}',
                                          ),
                                          trailing: const Icon(
                                            Icons.chevron_right,
                                          ),
                                          onTap: () => _pickCard(card),
                                        );
                                      },
                                    ),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ),
                ),
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
                        onPressed: () {
                          // Stay on page; user can use search below.
                        },
                        icon: const Icon(Icons.search),
                        label: const Text('No, search instead'),
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
                          separatorBuilder: (_, __) => const Divider(height: 1),
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
