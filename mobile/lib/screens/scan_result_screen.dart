import 'package:flutter/material.dart';
import '../models/card.dart';
import '../services/api_service.dart';

class ScanResultScreen extends StatefulWidget {
  final List<CardModel> cards;
  final String searchQuery;

  const ScanResultScreen({
    super.key,
    required this.cards,
    required this.searchQuery,
  });

  @override
  State<ScanResultScreen> createState() => _ScanResultScreenState();
}

class _ScanResultScreenState extends State<ScanResultScreen> {
  final ApiService _apiService = ApiService();
  int _quantity = 1;
  String _condition = 'NM';
  bool _foil = false;
  bool _isAdding = false;

  final List<String> _conditions = ['M', 'NM', 'EX', 'GD', 'LP', 'PL', 'PR'];

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
              // Foil
              SwitchListTile(
                title: const Text('Foil'),
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
    );
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
          : ListView.builder(
              itemCount: widget.cards.length,
              itemBuilder: (context, index) {
                final card = widget.cards[index];
                return Card(
                  margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  child: ListTile(
                    leading: card.imageUrl != null
                        ? ClipRRect(
                            borderRadius: BorderRadius.circular(4),
                            child: Image.network(
                              card.imageUrl!,
                              width: 50,
                              fit: BoxFit.cover,
                              errorBuilder: (_, __, ___) => const Icon(Icons.image),
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
    );
  }
}
