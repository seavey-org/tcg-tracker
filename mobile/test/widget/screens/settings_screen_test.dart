import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:mobile/screens/settings_screen.dart';
import 'package:mobile/services/api_service.dart';

class MockApiService extends Mock implements ApiService {}

void main() {
  late MockApiService mockApiService;

  setUp(() {
    mockApiService = MockApiService();
  });

  Widget createWidget() {
    return MaterialApp(
      home: SettingsScreen(apiService: mockApiService),
    );
  }

  group('SettingsScreen', () {
    testWidgets('displays saved URL after loading', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://saved-server:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      final textField = tester.widget<TextFormField>(find.byType(TextFormField));
      expect(textField.controller?.text, 'http://saved-server:8080');
    });

    testWidgets('displays default URL when no saved URL', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      final textField = tester.widget<TextFormField>(find.byType(TextFormField));
      expect(textField.controller?.text, 'http://localhost:8080');
    });

    testWidgets('shows success snackbar when save succeeds', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );
      when(() => mockApiService.setServerUrl(any())).thenAnswer(
        (_) async {},
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      expect(find.text('Settings saved!'), findsOneWidget);
      expect(find.byType(SnackBar), findsOneWidget);
    });

    testWidgets('shows error snackbar when URL is empty', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      // Clear the text field
      await tester.enterText(find.byType(TextFormField), '');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      expect(find.text('Server URL cannot be empty'), findsOneWidget);
      verifyNever(() => mockApiService.setServerUrl(any()));
    });

    testWidgets('displays Server Configuration header', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      expect(find.text('Server Configuration'), findsOneWidget);
    });

    testWidgets('displays Settings app bar title', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      expect(find.text('Settings'), findsOneWidget);
    });

    testWidgets('displays version info', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      expect(find.text('TCG Tracker Mobile'), findsOneWidget);
      expect(find.text('Version 1.0.0'), findsOneWidget);
    });

    testWidgets('calls setServerUrl with entered URL', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );
      when(() => mockApiService.setServerUrl(any())).thenAnswer(
        (_) async {},
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextFormField), 'http://newserver:9090');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      verify(() => mockApiService.setServerUrl('http://newserver:9090')).called(1);
    });

    testWidgets('trims whitespace from URL before saving', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );
      when(() => mockApiService.setServerUrl(any())).thenAnswer(
        (_) async {},
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextFormField), '  http://server:8080  ');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      verify(() => mockApiService.setServerUrl('http://server:8080')).called(1);
    });

    testWidgets('has correct text field decoration', (tester) async {
      when(() => mockApiService.getServerUrl()).thenAnswer(
        (_) async => 'http://localhost:8080',
      );

      await tester.pumpWidget(createWidget());
      await tester.pumpAndSettle();

      expect(find.text('Server URL'), findsOneWidget);
      expect(find.byIcon(Icons.link), findsOneWidget);
    });
  });
}
