import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import '../models/card.dart';

class ApiService {
  static const String _serverUrlKey = 'server_url';
  static const String _defaultServerUrl = 'http://localhost:8080';

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

    final response = await http.get(uri).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      return CardSearchResult.fromJson(json.decode(response.body));
    } else {
      final error = json.decode(response.body);
      throw Exception(error['error'] ?? 'Failed to search cards');
    }
  }

  Future<List<CardModel>> identifyCard(String text, String game) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/identify');

    final response = await http.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: json.encode({'text': text, 'game': game}),
    ).timeout(
      const Duration(seconds: 35),
      onTimeout: () => throw Exception('Request timed out'),
    );

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return (data['cards'] as List<dynamic>)
          .map((c) => CardModel.fromJson(c as Map<String, dynamic>))
          .toList();
    } else {
      final error = json.decode(response.body);
      throw Exception(error['error'] ?? 'Failed to identify card');
    }
  }

  Future<void> addToCollection(
    String cardId, {
    int quantity = 1,
    String condition = 'NM',
    bool foil = false,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection');

    final response = await http.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: json.encode({
        'card_id': cardId,
        'quantity': quantity,
        'condition': condition,
        'foil': foil,
      }),
    );

    if (response.statusCode != 200 && response.statusCode != 201) {
      final error = json.decode(response.body);
      throw Exception(error['error'] ?? 'Failed to add to collection');
    }
  }
}
