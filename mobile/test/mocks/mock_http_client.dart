import 'package:http/http.dart' as http;
import 'package:mocktail/mocktail.dart';

/// Mock HTTP Client for testing API calls
class MockHttpClient extends Mock implements http.Client {}

/// Fake Uri for mocktail fallback registration
class FakeUri extends Fake implements Uri {}

/// Setup function to register fallbacks - call this in setUpAll
void registerHttpFallbacks() {
  registerFallbackValue(FakeUri());
}

/// Extension to provide common stub setups for MockHttpClient
extension MockHttpClientExtension on MockHttpClient {
  /// Stubs a GET request to return a successful response
  void stubGetSuccess(String body, {int statusCode = 200}) {
    when(() => get(any(), headers: any(named: 'headers')))
        .thenAnswer((_) async => http.Response(body, statusCode));
  }

  /// Stubs a GET request to return an error response
  void stubGetError(String body, {int statusCode = 500}) {
    when(() => get(any(), headers: any(named: 'headers')))
        .thenAnswer((_) async => http.Response(body, statusCode));
  }

  /// Stubs a GET request to throw an exception
  void stubGetException(Exception exception) {
    when(() => get(any(), headers: any(named: 'headers'))).thenThrow(exception);
  }

  /// Stubs a POST request to return a successful response
  void stubPostSuccess(String body, {int statusCode = 200}) {
    when(() => post(
          any(),
          headers: any(named: 'headers'),
          body: any(named: 'body'),
          encoding: any(named: 'encoding'),
        )).thenAnswer((_) async => http.Response(body, statusCode));
  }

  /// Stubs a POST request to return an error response
  void stubPostError(String body, {int statusCode = 500}) {
    when(() => post(
          any(),
          headers: any(named: 'headers'),
          body: any(named: 'body'),
          encoding: any(named: 'encoding'),
        )).thenAnswer((_) async => http.Response(body, statusCode));
  }

  /// Stubs a POST request to throw an exception
  void stubPostException(Exception exception) {
    when(() => post(
          any(),
          headers: any(named: 'headers'),
          body: any(named: 'body'),
          encoding: any(named: 'encoding'),
        )).thenThrow(exception);
  }

  /// Stubs a POST request with specific URL matching
  void stubPostForUrl(Uri url, String body, {int statusCode = 200}) {
    when(() => post(
          url,
          headers: any(named: 'headers'),
          body: any(named: 'body'),
          encoding: any(named: 'encoding'),
        )).thenAnswer((_) async => http.Response(body, statusCode));
  }
}
