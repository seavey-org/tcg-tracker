import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../models/collection_stats.dart';
import '../models/price_status.dart';

class ApiService {
  static const String _serverUrlKey = 'server_url';
  static const String _defaultServerUrl = 'http://localhost:8080';

  final http.Client _httpClient;

  ApiService({http.Client? httpClient}) : _httpClient = httpClient ?? http.Client();

  /// Safely decode JSON, returning a map with error key if decoding fails
  Map<String, dynamic> _safeJsonDecode(String body) {
    try {
      return json.decode(body) as Map<String, dynamic>;
    } catch (e) {
      return {'error': 'Server error: ${body.substring(0, body.length.clamp(0, 100))}'};
    }
  }

  Future<String> getServerUrl() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_serverUrlKey) ?? _defaultServerUrl;
  }

  Future<void> setServerUrl(String url) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_serverUrlKey, url);
  }

  Future<CardSearchResult> searchCards(String query, String game) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/search')
        .replace(queryParameters: {'q': query, 'game': game});

    final response = await _httpClient.get(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      return CardSearchResult.fromJson(json.decode(response.body));
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to search cards');
    }
  }

  Future<ScanResult> identifyCard(String text, String game) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/identify');

    final response = await _httpClient.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: json.encode({'text': text, 'game': game}),
    ).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return ScanResult.fromJson(data);
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to identify card');
    }
  }

  Future<CollectionItem> addToCollection(
    String cardId, {
    int quantity = 1,
    String condition = 'NM',
    bool foil = false,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection');

    final response = await _httpClient.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: json.encode({
        'card_id': cardId,
        'quantity': quantity,
        'condition': condition,
        'foil': foil,
      }),
    ).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode != 200 && response.statusCode != 201) {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to add to collection');
    }

    return CollectionItem.fromJson(json.decode(response.body));
  }

  /// Get all collection items, optionally filtered by game
  Future<List<CollectionItem>> getCollection({String? game}) async {
    final serverUrl = await getServerUrl();
    var uri = Uri.parse('$serverUrl/api/collection');
    if (game != null && game.isNotEmpty) {
      uri = uri.replace(queryParameters: {'game': game});
    }

    final response = await _httpClient.get(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      final List<dynamic> data = json.decode(response.body);
      return data
          .map((item) => CollectionItem.fromJson(item as Map<String, dynamic>))
          .toList();
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to get collection');
    }
  }

  /// Get collection statistics
  Future<CollectionStats> getStats() async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/stats');

    final response = await _httpClient.get(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      return CollectionStats.fromJson(json.decode(response.body));
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to get stats');
    }
  }

  /// Update a collection item
  Future<CollectionItem> updateCollectionItem(
    int id, {
    int? quantity,
    String? condition,
    bool? foil,
    String? notes,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/$id');

    final body = <String, dynamic>{};
    if (quantity != null) body['quantity'] = quantity;
    if (condition != null) body['condition'] = condition;
    if (foil != null) body['foil'] = foil;
    if (notes != null) body['notes'] = notes;

    final response = await _httpClient.put(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: json.encode(body),
    ).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      return CollectionItem.fromJson(json.decode(response.body));
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to update item');
    }
  }

  /// Delete a collection item
  Future<void> deleteCollectionItem(int id) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/$id');

    final response = await _httpClient.delete(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode != 200) {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to delete item');
    }
  }

  /// Trigger bulk price refresh for collection
  Future<int> refreshAllPrices() async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/refresh-prices');

    final response = await _httpClient.post(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return data['updated'] ?? 0;
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to refresh prices');
    }
  }

  /// Get price API status (quota info)
  Future<PriceStatus> getPriceStatus() async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/prices/status');

    final response = await _httpClient.get(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      return PriceStatus.fromJson(json.decode(response.body));
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to get price status');
    }
  }

  /// Refresh price for a single card
  Future<CardModel> refreshCardPrice(String cardId) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/$cardId/refresh-price');

    final response = await _httpClient.post(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return CardModel.fromJson(data['card'] ?? data);
    } else if (response.statusCode == 429) {
      throw Exception('Price API quota exceeded. Try again tomorrow.');
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to refresh price');
    }
  }

  /// Test connection to the server
  Future<bool> testConnection() async {
    try {
      final serverUrl = await getServerUrl();
      final uri = Uri.parse('$serverUrl/health');
      final response = await _httpClient.get(uri).timeout(
        const Duration(seconds: 10),
        onTimeout: () => throw Exception('Connection timed out'),
      );
      return response.statusCode == 200;
    } catch (e) {
      return false;
    }
  }
}
