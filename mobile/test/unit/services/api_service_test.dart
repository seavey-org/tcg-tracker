import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:mobile/services/api_service.dart';
import '../../fixtures/card_fixtures.dart';
import '../../fixtures/scan_fixtures.dart';

class MockHttpClient extends Mock implements http.Client {}

class FakeUri extends Fake implements Uri {}

void main() {
  late ApiService apiService;
  late MockHttpClient mockHttpClient;

  setUpAll(() {
    registerFallbackValue(FakeUri());
  });

  setUp(() {
    mockHttpClient = MockHttpClient();
    apiService = ApiService(httpClient: mockHttpClient);
    SharedPreferences.setMockInitialValues({});
  });

  group('ApiService', () {
    group('getServerUrl', () {
      test('returns stored URL when available', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        final url = await service.getServerUrl();

        expect(url, 'http://test:8080');
      });

      test('returns default localhost:8080 when no URL stored', () async {
        SharedPreferences.setMockInitialValues({});
        final service = ApiService(httpClient: mockHttpClient);

        final url = await service.getServerUrl();

        expect(url, 'http://localhost:8080');
      });
    });

    group('setServerUrl', () {
      test('stores the URL in SharedPreferences', () async {
        SharedPreferences.setMockInitialValues({});
        final service = ApiService(httpClient: mockHttpClient);

        await service.setServerUrl('http://newserver:9090');

        final storedUrl = await service.getServerUrl();
        expect(storedUrl, 'http://newserver:9090');
      });
    });

    group('searchCards', () {
      test('returns CardSearchResult on success', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.get(any())).thenAnswer(
          (_) async => http.Response(
            json.encode(CardFixtures.searchResultJson),
            200,
          ),
        );

        final result = await service.searchCards('charizard', 'pokemon');

        expect(result.cards.length, 2);
        expect(result.cards[0].name, 'Charizard VMAX');
        expect(result.totalCount, 2);
      });

      test('throws exception with error message on failure', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.get(any())).thenAnswer(
          (_) async => http.Response(
            json.encode({'error': 'Card not found'}),
            404,
          ),
        );

        expect(
          () => service.searchCards('nonexistent', 'pokemon'),
          throwsA(isA<Exception>().having(
            (e) => e.toString(),
            'message',
            contains('Card not found'),
          )),
        );
      });

      test('uses default error message when error field missing', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.get(any())).thenAnswer(
          (_) async => http.Response(
            json.encode({}),
            500,
          ),
        );

        expect(
          () => service.searchCards('test', 'mtg'),
          throwsA(isA<Exception>().having(
            (e) => e.toString(),
            'message',
            contains('Failed to search cards'),
          )),
        );
      });

      test('constructs correct URL with query parameters', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
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

    group('identifyCard', () {
      test('returns ScanResult on success', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer(
          (_) async => http.Response(
            json.encode(ScanFixtures.completeScanResultJson),
            200,
          ),
        );

        final result = await service.identifyCard('Charizard VMAX\n025/185', 'pokemon');

        expect(result.cards.length, 2);
        expect(result.metadata.cardName, 'Charizard VMAX');
        expect(result.metadata.confidence, 0.85);
      });

      test('throws exception with error message on failure', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer(
          (_) async => http.Response(
            json.encode({'error': 'OCR parsing failed'}),
            400,
          ),
        );

        expect(
          () => service.identifyCard('garbled text', 'pokemon'),
          throwsA(isA<Exception>().having(
            (e) => e.toString(),
            'message',
            contains('OCR parsing failed'),
          )),
        );
      });

      test('sends correct request body', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        String? capturedBody;
        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer((invocation) async {
          capturedBody = invocation.namedArguments[const Symbol('body')] as String;
          return http.Response(
            json.encode(ScanFixtures.emptyScanResultJson),
            200,
          );
        });

        await service.identifyCard('Test OCR Text', 'mtg');

        final decodedBody = json.decode(capturedBody!);
        expect(decodedBody['text'], 'Test OCR Text');
        expect(decodedBody['game'], 'mtg');
      });

      test('sends Content-Type header', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        Map<String, String>? capturedHeaders;
        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer((invocation) async {
          capturedHeaders = invocation.namedArguments[const Symbol('headers')]
              as Map<String, String>;
          return http.Response(
            json.encode(ScanFixtures.emptyScanResultJson),
            200,
          );
        });

        await service.identifyCard('Test', 'pokemon');

        expect(capturedHeaders?['Content-Type'], 'application/json');
      });
    });

    group('addToCollection', () {
      test('completes successfully on 200 response', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer(
          (_) async => http.Response('{}', 200),
        );

        await expectLater(
          service.addToCollection('card-123'),
          completes,
        );
      });

      test('completes successfully on 201 response', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer(
          (_) async => http.Response('{}', 201),
        );

        await expectLater(
          service.addToCollection('card-123'),
          completes,
        );
      });

      test('throws exception with error message on failure', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer(
          (_) async => http.Response(
            json.encode({'error': 'Card already in collection'}),
            409,
          ),
        );

        expect(
          () => service.addToCollection('card-123'),
          throwsA(isA<Exception>().having(
            (e) => e.toString(),
            'message',
            contains('Card already in collection'),
          )),
        );
      });

      test('sends correct request body with default values', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        String? capturedBody;
        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer((invocation) async {
          capturedBody = invocation.namedArguments[const Symbol('body')] as String;
          return http.Response('{}', 200);
        });

        await service.addToCollection('card-123');

        final decodedBody = json.decode(capturedBody!);
        expect(decodedBody['card_id'], 'card-123');
        expect(decodedBody['quantity'], 1);
        expect(decodedBody['condition'], 'NM');
        expect(decodedBody['foil'], false);
      });

      test('sends correct request body with custom values', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        String? capturedBody;
        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer((invocation) async {
          capturedBody = invocation.namedArguments[const Symbol('body')] as String;
          return http.Response('{}', 200);
        });

        await service.addToCollection(
          'card-456',
          quantity: 3,
          condition: 'LP',
          foil: true,
        );

        final decodedBody = json.decode(capturedBody!);
        expect(decodedBody['card_id'], 'card-456');
        expect(decodedBody['quantity'], 3);
        expect(decodedBody['condition'], 'LP');
        expect(decodedBody['foil'], true);
      });

      test('uses correct endpoint URL', () async {
        SharedPreferences.setMockInitialValues({
          'server_url': 'http://test:8080',
        });
        final service = ApiService(httpClient: mockHttpClient);

        Uri? capturedUri;
        when(() => mockHttpClient.post(
              any(),
              headers: any(named: 'headers'),
              body: any(named: 'body'),
            )).thenAnswer((invocation) async {
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
