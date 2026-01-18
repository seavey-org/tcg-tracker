import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/card.dart';
import '../models/collection_item.dart';
import '../models/collection_stats.dart';
import '../models/price_status.dart';
import 'image_analysis_service.dart';

class ApiService {
  static const String _serverUrlKey = 'server_url';
  static const String _defaultServerUrl = 'https://tcg.seavey.dev';

  // Use legacy key for migration from SharedPreferences
  static const String _legacyServerUrlKey = 'server_url';

  final http.Client _httpClient;
  final FlutterSecureStorage _secureStorage;

  ApiService({http.Client? httpClient, FlutterSecureStorage? secureStorage})
    : _httpClient = httpClient ?? http.Client(),
      _secureStorage = secureStorage ?? const FlutterSecureStorage();

  /// Safely decode JSON, returning a map with error key if decoding fails
  Map<String, dynamic> _safeJsonDecode(String body) {
    try {
      return json.decode(body) as Map<String, dynamic>;
    } catch (e) {
      return {
        'error':
            'Server error: ${body.substring(0, body.length.clamp(0, 100))}',
      };
    }
  }

  Future<String> getServerUrl() async {
    // First, try to get from secure storage
    String? serverUrl = await _secureStorage.read(key: _serverUrlKey);

    if (serverUrl != null) {
      return serverUrl;
    }

    // Migration: Check if URL exists in SharedPreferences (legacy storage)
    final prefs = await SharedPreferences.getInstance();
    final legacyUrl = prefs.getString(_legacyServerUrlKey);

    if (legacyUrl != null) {
      // Migrate to secure storage
      await _secureStorage.write(key: _serverUrlKey, value: legacyUrl);
      // Remove from legacy storage
      await prefs.remove(_legacyServerUrlKey);
      return legacyUrl;
    }

    return _defaultServerUrl;
  }

  Future<void> setServerUrl(String url) async {
    await _secureStorage.write(key: _serverUrlKey, value: url);

    // Also clear from legacy storage if it exists (migration cleanup)
    final prefs = await SharedPreferences.getInstance();
    if (prefs.containsKey(_legacyServerUrlKey)) {
      await prefs.remove(_legacyServerUrlKey);
    }
  }

  Future<CardSearchResult> searchCards(
    String query,
    String game, {
    List<String>? setIDs,
  }) async {
    final serverUrl = await getServerUrl();

    final params = <String, String>{'q': query, 'game': game};

    if (setIDs != null && setIDs.isNotEmpty) {
      params['set_ids'] = setIDs.join(',');
    }

    final uri = Uri.parse(
      '$serverUrl/api/cards/search',
    ).replace(queryParameters: params);

    final response = await _httpClient
        .get(uri)
        .timeout(
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

  Future<ScanResult> identifyCard(
    String text,
    String game, {
    ImageAnalysisResult? imageAnalysis,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/identify');

    final body = <String, dynamic>{'text': text, 'game': game};

    // Include image analysis if provided
    if (imageAnalysis != null) {
      body['image_analysis'] = imageAnalysis.toJson();
    }

    final response = await _httpClient
        .post(
          uri,
          headers: {'Content-Type': 'application/json'},
          body: json.encode(body),
        )
        .timeout(
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
    bool firstEdition = false,
    List<int>? scannedImageBytes,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection');

    final body = <String, dynamic>{
      'card_id': cardId,
      'quantity': quantity,
      'condition': condition,
      'foil': foil,
      'first_edition': firstEdition,
    };

    // Include scanned image if provided
    if (scannedImageBytes != null && scannedImageBytes.isNotEmpty) {
      body['scanned_image_data'] = base64Encode(scannedImageBytes);
    }

    final response = await _httpClient
        .post(
          uri,
          headers: {'Content-Type': 'application/json'},
          body: json.encode(body),
        )
        .timeout(
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

    final response = await _httpClient
        .get(uri)
        .timeout(
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

    final response = await _httpClient
        .get(uri)
        .timeout(
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
    bool? firstEdition,
    String? notes,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/$id');

    final body = <String, dynamic>{};
    if (quantity != null) body['quantity'] = quantity;
    if (condition != null) body['condition'] = condition;
    if (foil != null) body['foil'] = foil;
    if (firstEdition != null) body['first_edition'] = firstEdition;
    if (notes != null) body['notes'] = notes;

    final response = await _httpClient
        .put(
          uri,
          headers: {'Content-Type': 'application/json'},
          body: json.encode(body),
        )
        .timeout(
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

    final response = await _httpClient
        .delete(uri)
        .timeout(
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

    final response = await _httpClient
        .post(uri)
        .timeout(
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

    final response = await _httpClient
        .get(uri)
        .timeout(
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

    final response = await _httpClient
        .post(uri)
        .timeout(
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
      final response = await _httpClient
          .get(uri)
          .timeout(
            const Duration(seconds: 10),
            onTimeout: () => throw Exception('Connection timed out'),
          );
      return response.statusCode == 200;
    } catch (e) {
      return false;
    }
  }

  /// Check if server-side OCR is available
  Future<bool> isServerOCRAvailable() async {
    try {
      final serverUrl = await getServerUrl();
      final uri = Uri.parse('$serverUrl/api/cards/ocr-status');
      final response = await _httpClient
          .get(uri)
          .timeout(
            const Duration(seconds: 10),
            onTimeout: () => throw Exception('Request timed out'),
          );

      if (response.statusCode == 200) {
        final data = json.decode(response.body);
        return data['server_ocr_available'] ?? false;
      }
      return false;
    } catch (e) {
      return false;
    }
  }

  /// Identify card from an image using server-side OCR
  /// This is an alternative to client-side ML Kit OCR
  Future<ScanResult> identifyCardFromImage(
    List<int> imageBytes,
    String game,
  ) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/identify-image');

    // Create multipart request
    final request = http.MultipartRequest('POST', uri);
    request.fields['game'] = game;
    request.files.add(
      http.MultipartFile.fromBytes(
        'image',
        imageBytes,
        filename: 'card_image.jpg',
      ),
    );

    final streamedResponse = await request.send().timeout(
      const Duration(seconds: 60), // Longer timeout for image processing
      onTimeout: () => throw Exception('Request timed out'),
    );

    final response = await http.Response.fromStream(streamedResponse);

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return ScanResult.fromJson(data);
    } else if (response.statusCode == 503) {
      throw Exception('Server-side OCR is not available');
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to identify card from image');
    }
  }

  /// Identify set from an image (set icon matching only, no OCR)
  /// This can be used in parallel with client-side OCR to improve card matching
  Future<SetIdentificationResult?> identifySetFromImage(
    List<int> imageBytes,
    String game,
  ) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/identify-set');

    // Create multipart request
    final request = http.MultipartRequest('POST', uri);
    request.fields['game'] = game;
    request.files.add(
      http.MultipartFile.fromBytes(
        'image',
        imageBytes,
        filename: 'card_image.jpg',
      ),
    );

    final streamedResponse = await request.send().timeout(
      const Duration(seconds: 30),
      onTimeout: () => throw Exception('Request timed out'),
    );

    final response = await http.Response.fromStream(streamedResponse);

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return SetIdentificationResult.fromJson(data);
    } else if (response.statusCode == 503) {
      // Service unavailable - return null to indicate set ID not available
      return null;
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to identify set from image');
    }
  }
}

/// Result of set identification from image
class SetIdentificationResult {
  final String bestSetId;
  final double confidence;
  final bool lowConfidence;
  final List<SetCandidate> candidates;

  SetIdentificationResult({
    required this.bestSetId,
    required this.confidence,
    required this.lowConfidence,
    required this.candidates,
  });

  factory SetIdentificationResult.fromJson(Map<String, dynamic> json) {
    return SetIdentificationResult(
      bestSetId: json['best_set_id'] ?? '',
      confidence: (json['confidence'] ?? 0.0).toDouble(),
      lowConfidence: json['low_confidence'] ?? true,
      candidates: (json['candidates'] as List<dynamic>?)
              ?.map((c) => SetCandidate.fromJson(c as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'best_set_id': bestSetId,
      'confidence': confidence,
      'low_confidence': lowConfidence,
      'candidates': candidates.map((c) => c.toJson()).toList(),
    };
  }
}

/// A candidate set from set identification
class SetCandidate {
  final String setId;
  final double score;

  SetCandidate({required this.setId, required this.score});

  factory SetCandidate.fromJson(Map<String, dynamic> json) {
    return SetCandidate(
      setId: json['set_id'] ?? '',
      score: (json['score'] ?? 0.0).toDouble(),
    );
  }

  Map<String, dynamic> toJson() {
    return {'set_id': setId, 'score': score};
  }
}
