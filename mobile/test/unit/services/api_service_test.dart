import 'dart:convert';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:mobile/models/collection_item.dart' show PrintingType;
import 'package:mobile/services/api_service.dart';
import '../../fixtures/card_fixtures.dart';

class MockHttpClient extends Mock implements http.Client {}

class FakeUri extends Fake implements Uri {}

// In-memory storage for secure storage mock
final Map<String, String> _secureStorageValues = {};

void main() {
  // Initialize binding for flutter_secure_storage
  TestWidgetsFlutterBinding.ensureInitialized();

  late ApiService apiService;
  late MockHttpClient mockHttpClient;

  setUpAll(() {
    registerFallbackValue(FakeUri());
    // Mock the secure storage method channel with in-memory storage
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
          (MethodCall methodCall) async {
            final args = methodCall.arguments as Map<dynamic, dynamic>?;
            final key = args?['key'] as String?;

            switch (methodCall.method) {
              case 'read':
                return _secureStorageValues[key];
              case 'write':
                final value = args?['value'] as String?;
                if (key != null && value != null) {
                  _secureStorageValues[key] = value;
                }
                return null;
              case 'delete':
                if (key != null) {
                  _secureStorageValues.remove(key);
                }
                return null;
              case 'deleteAll':
                _secureStorageValues.clear();
                return null;
              default:
                return null;
            }
          },
        );
  });

  setUp(() {
    // Clear secure storage before each test
    _secureStorageValues.clear();
    mockHttpClient = MockHttpClient();
    apiService = ApiService(httpClient: mockHttpClient);
    SharedPreferences.setMockInitialValues({});
  });

  group('ApiService', () {
    group('getServerUrl', () {
      test('returns stored URL when available', () async {
        // Pre-populate secure storage
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        final url = await service.getServerUrl();

        expect(url, 'http://test:8080');
      });

      test('returns default production URL when no URL stored', () async {
        final url = await apiService.getServerUrl();

        expect(url, 'https://tcg.seavey.dev');
      });
    });

    group('setServerUrl', () {
      test('stores the URL in secure storage', () async {
        await apiService.setServerUrl('http://newserver:9090');

        final storedUrl = await apiService.getServerUrl();
        expect(storedUrl, 'http://newserver:9090');
      });
    });

    group('searchCards', () {
      test('returns CardSearchResult on success', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.get(any())).thenAnswer(
          (_) async =>
              http.Response(json.encode(CardFixtures.searchResultJson), 200),
        );

        final result = await service.searchCards('charizard', 'pokemon');

        expect(result.cards.length, 2);
        expect(result.cards[0].name, 'Charizard VMAX');
        expect(result.totalCount, 2);
      });

      test('throws exception with error message on failure', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.get(any())).thenAnswer(
          (_) async =>
              http.Response(json.encode({'error': 'Card not found'}), 404),
        );

        expect(
          () => service.searchCards('nonexistent', 'pokemon'),
          throwsA(
            isA<Exception>().having(
              (e) => e.toString(),
              'message',
              contains('Card not found'),
            ),
          ),
        );
      });

      test('uses default error message when error field missing', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        when(
          () => mockHttpClient.get(any()),
        ).thenAnswer((_) async => http.Response(json.encode({}), 500));

        expect(
          () => service.searchCards('test', 'mtg'),
          throwsA(
            isA<Exception>().having(
              (e) => e.toString(),
              'message',
              contains('Failed to search cards'),
            ),
          ),
        );
      });

      test('constructs correct URL with query parameters', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        Uri? capturedUri;
        when(() => mockHttpClient.get(any())).thenAnswer((invocation) async {
          capturedUri = invocation.positionalArguments[0] as Uri;
          return http.Response(
            json.encode(CardFixtures.emptySearchResultJson),
            200,
          );
        });

        await service.searchCards('pikachu', 'pokemon');

        expect(capturedUri?.host, 'test');
        expect(capturedUri?.port, 8080);
        expect(capturedUri?.path, '/api/cards/search');
        expect(capturedUri?.queryParameters['q'], 'pikachu');
        expect(capturedUri?.queryParameters['game'], 'pokemon');
      });
    });

    group('addToCollection', () {
      test('completes successfully on 200 response', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        when(
          () => mockHttpClient.post(
            any(),
            headers: any(named: 'headers'),
            body: any(named: 'body'),
          ),
        ).thenAnswer((_) async => http.Response('{}', 200));

        await expectLater(service.addToCollection('card-123'), completes);
      });

      test('completes successfully on 201 response', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        when(
          () => mockHttpClient.post(
            any(),
            headers: any(named: 'headers'),
            body: any(named: 'body'),
          ),
        ).thenAnswer((_) async => http.Response('{}', 201));

        await expectLater(service.addToCollection('card-123'), completes);
      });

      test('throws exception with error message on failure', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        when(
          () => mockHttpClient.post(
            any(),
            headers: any(named: 'headers'),
            body: any(named: 'body'),
          ),
        ).thenAnswer(
          (_) async => http.Response(
            json.encode({'error': 'Card already in collection'}),
            409,
          ),
        );

        expect(
          () => service.addToCollection('card-123'),
          throwsA(
            isA<Exception>().having(
              (e) => e.toString(),
              'message',
              contains('Card already in collection'),
            ),
          ),
        );
      });

      test('sends correct request body with default values', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        String? capturedBody;
        when(
          () => mockHttpClient.post(
            any(),
            headers: any(named: 'headers'),
            body: any(named: 'body'),
          ),
        ).thenAnswer((invocation) async {
          capturedBody =
              invocation.namedArguments[const Symbol('body')] as String;
          return http.Response('{}', 200);
        });

        await service.addToCollection('card-123');

        final decodedBody = json.decode(capturedBody!);
        expect(decodedBody['card_id'], 'card-123');
        expect(decodedBody['quantity'], 1);
        expect(decodedBody['condition'], 'NM');
        expect(decodedBody['printing'], 'Normal');
      });

      test('sends correct request body with custom values', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        String? capturedBody;
        when(
          () => mockHttpClient.post(
            any(),
            headers: any(named: 'headers'),
            body: any(named: 'body'),
          ),
        ).thenAnswer((invocation) async {
          capturedBody =
              invocation.namedArguments[const Symbol('body')] as String;
          return http.Response('{}', 200);
        });

        await service.addToCollection(
          'card-456',
          quantity: 3,
          condition: 'LP',
          printing: PrintingType.foil,
        );

        final decodedBody = json.decode(capturedBody!);
        expect(decodedBody['card_id'], 'card-456');
        expect(decodedBody['quantity'], 3);
        expect(decodedBody['condition'], 'LP');
        expect(decodedBody['printing'], 'Foil');
      });

      test('uses correct endpoint URL', () async {
        _secureStorageValues['server_url'] = 'http://test:8080';
        final service = ApiService(httpClient: mockHttpClient);

        Uri? capturedUri;
        when(
          () => mockHttpClient.post(
            any(),
            headers: any(named: 'headers'),
            body: any(named: 'body'),
          ),
        ).thenAnswer((invocation) async {
          capturedUri = invocation.positionalArguments[0] as Uri;
          return http.Response('{}', 200);
        });

        await service.addToCollection('card-123');

        expect(capturedUri?.host, 'test');
        expect(capturedUri?.port, 8080);
        expect(capturedUri?.path, '/api/collection');
      });
    });

    group('constructor', () {
      test('creates default HTTP client when not provided', () {
        final service = ApiService();
        expect(service, isNotNull);
      });

      test('uses provided HTTP client', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });

        when(() => mockHttpClient.get(any())).thenAnswer(
          (_) async => http.Response(
            json.encode(CardFixtures.emptySearchResultJson),
            200,
          ),
        );

        final result = await apiService.searchCards('test', 'mtg');

        verify(() => mockHttpClient.get(any())).called(1);
        expect(result.cards, isEmpty);
      });
    });
  });
}
