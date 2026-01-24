import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';
import 'package:http/http.dart' as http;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:image/image.dart' as img;
import '../models/card.dart';
import '../models/collection_item.dart' show CollectionItem, PrintingType;
import '../models/collection_stats.dart';
import '../models/gemini_scan_result.dart';
import '../models/grouped_collection.dart';
import '../models/price_status.dart';
import 'auth_service.dart';

class ApiService {
  static const String _serverUrlKey = 'server_url';
  static const String _defaultServerUrl = 'https://tcg.seavey.dev';

  // Use legacy key for migration from SharedPreferences
  static const String _legacyServerUrlKey = 'server_url';

  // Maximum image dimension for upload (matches server expectation)
  static const int _maxImageDimension = 1280;

  final http.Client _httpClient;
  final FlutterSecureStorage _secureStorage;
  final AuthService _authService;

  // Callback for when auth is required (401 response)
  void Function()? onAuthRequired;

  ApiService({
    http.Client? httpClient,
    FlutterSecureStorage? secureStorage,
    AuthService? authService,
  }) : _httpClient = httpClient ?? http.Client(),
       _secureStorage = secureStorage ?? const FlutterSecureStorage(),
       _authService = authService ?? AuthService();

  /// Get the auth service for direct access
  AuthService get authService => _authService;

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

  Future<CollectionItem> addToCollection(
    String cardId, {
    int quantity = 1,
    String condition = 'NM',
    PrintingType printing = PrintingType.normal,
    List<int>? scannedImageBytes,
    String? language, // e.g., "Japanese", "English", "German"
    String? ocrText, // OCR text for caching Japanese card translations
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection');

    final body = <String, dynamic>{
      'card_id': cardId,
      'quantity': quantity,
      'condition': condition,
      'printing': printing.value,
    };

    // Include language if specified (for Japanese/foreign cards with different pricing)
    if (language != null && language.isNotEmpty) {
      body['language'] = language;
    }

    // Include scanned image if provided
    if (scannedImageBytes != null && scannedImageBytes.isNotEmpty) {
      body['scanned_image_data'] = base64Encode(scannedImageBytes);
    }

    // Include OCR text for caching Japanese card translations
    // This allows instant lookup on future scans of the same card
    if (ocrText != null && ocrText.isNotEmpty) {
      body['ocr_text'] = ocrText;
    }

    // Get auth headers for protected endpoint
    final authHeaders = await _authService.getAuthHeaders();

    final response = await _httpClient
        .post(
          uri,
          headers: {'Content-Type': 'application/json', ...authHeaders},
          body: json.encode(body),
        )
        .timeout(
          const Duration(seconds: 35),
          onTimeout: () => throw Exception('Request timed out'),
        );

    if (response.statusCode == 401) {
      onAuthRequired?.call();
      throw AuthRequiredException('Admin access required to add cards');
    }

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

  /// Get collection items grouped by card
  Future<List<GroupedCollectionItem>> getGroupedCollection({
    String? game,
  }) async {
    final serverUrl = await getServerUrl();
    var uri = Uri.parse('$serverUrl/api/collection/grouped');
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
          .map(
            (item) =>
                GroupedCollectionItem.fromJson(item as Map<String, dynamic>),
          )
          .toList();
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to get grouped collection');
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
  /// Returns CollectionUpdateResponse with operation info (updated/split/merged)
  Future<CollectionUpdateResponse> updateCollectionItem(
    int id, {
    int? quantity,
    String? condition,
    PrintingType? printing,
    String? notes,
  }) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/$id');

    final body = <String, dynamic>{};
    if (quantity != null) body['quantity'] = quantity;
    if (condition != null) body['condition'] = condition;
    if (printing != null) body['printing'] = printing.value;
    if (notes != null) body['notes'] = notes;

    // Get auth headers for protected endpoint
    final authHeaders = await _authService.getAuthHeaders();

    final response = await _httpClient
        .put(
          uri,
          headers: {'Content-Type': 'application/json', ...authHeaders},
          body: json.encode(body),
        )
        .timeout(
          const Duration(seconds: 35),
          onTimeout: () => throw Exception('Request timed out'),
        );

    if (response.statusCode == 401) {
      onAuthRequired?.call();
      throw AuthRequiredException('Admin access required to update cards');
    }

    if (response.statusCode == 200) {
      return CollectionUpdateResponse.fromJson(json.decode(response.body));
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to update item');
    }
  }

  /// Delete a collection item
  Future<void> deleteCollectionItem(int id) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/$id');

    // Get auth headers for protected endpoint
    final authHeaders = await _authService.getAuthHeaders();

    final response = await _httpClient
        .delete(uri, headers: authHeaders)
        .timeout(
          const Duration(seconds: 35),
          onTimeout: () => throw Exception('Request timed out'),
        );

    if (response.statusCode == 401) {
      onAuthRequired?.call();
      throw AuthRequiredException('Admin access required to delete cards');
    }

    if (response.statusCode != 200) {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to delete item');
    }
  }

  /// Trigger bulk price refresh for collection
  Future<int> refreshAllPrices() async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/collection/refresh-prices');

    // Get auth headers for protected endpoint
    final authHeaders = await _authService.getAuthHeaders();

    final response = await _httpClient
        .post(uri, headers: authHeaders)
        .timeout(
          const Duration(seconds: 35),
          onTimeout: () => throw Exception('Request timed out'),
        );

    if (response.statusCode == 401) {
      onAuthRequired?.call();
      throw AuthRequiredException('Admin access required to refresh prices');
    }

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

  /// Downscale image to max dimension while preserving aspect ratio.
  /// Returns the original bytes if already small enough to avoid lossy re-compression.
  /// When resizing is needed, uses high quality (95) to preserve text clarity for OCR.
  Uint8List _downscaleImage(List<int> imageBytes) {
    final image = img.decodeImage(Uint8List.fromList(imageBytes));
    if (image == null) {
      // Can't decode, return original bytes and let server handle it
      return Uint8List.fromList(imageBytes);
    }

    final maxDim = max(image.width, image.height);
    if (maxDim <= _maxImageDimension) {
      // Already small enough - return ORIGINAL bytes to avoid lossy re-compression.
      // Each JPEG encode/decode cycle degrades quality, especially for text edges
      // which are critical for OCR accuracy.
      return Uint8List.fromList(imageBytes);
    }

    // Calculate new dimensions
    final scale = _maxImageDimension / maxDim;
    final newWidth = (image.width * scale).round();
    final newHeight = (image.height * scale).round();

    // Resize and encode as JPEG with HIGH quality (95) to preserve text clarity.
    // Lower quality causes JPEG artifacts that degrade OCR accuracy.
    final resized = img.copyResize(image, width: newWidth, height: newHeight);
    return Uint8List.fromList(img.encodeJpg(resized, quality: 95));
  }

  /// Identify card from an image using Gemini Vision
  /// Gemini automatically detects the game type (Pokemon/MTG) and language
  Future<GeminiScanResult> identifyCardFromImage(List<int> imageBytes) async {
    final serverUrl = await getServerUrl();
    final uri = Uri.parse('$serverUrl/api/cards/identify-image');

    // Downscale image for faster upload and processing
    final scaledBytes = _downscaleImage(imageBytes);

    // Create multipart request
    final request = http.MultipartRequest('POST', uri);
    request.files.add(
      http.MultipartFile.fromBytes(
        'image',
        scaledBytes,
        filename: 'card_image.jpg',
      ),
    );

    final streamedResponse = await request.send().timeout(
      const Duration(seconds: 90), // Longer timeout for Gemini multi-turn
      onTimeout: () => throw Exception('Request timed out'),
    );

    final response = await http.Response.fromStream(streamedResponse);

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return GeminiScanResult.fromJson(data);
    } else if (response.statusCode == 503) {
      throw Exception('Card identification service is not available');
    } else {
      final error = _safeJsonDecode(response.body);
      throw Exception(error['error'] ?? 'Failed to identify card from image');
    }
  }
}
