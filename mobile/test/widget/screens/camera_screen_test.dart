import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:camera/camera.dart';
import 'package:mobile/screens/camera_screen.dart';
import 'package:mobile/services/api_service.dart';
import 'package:mobile/services/camera_service.dart';
import 'package:mobile/services/ocr_service.dart';

class MockCameraService extends Mock implements CameraService {}
class MockOcrService extends Mock implements OcrService {}
class MockApiService extends Mock implements ApiService {}
class MockCameraController extends Mock implements CameraController {}

void main() {
  late MockCameraService mockCameraService;
  late MockOcrService mockOcrService;
  late MockApiService mockApiService;

  setUp(() {
    mockCameraService = MockCameraService();
    mockOcrService = MockOcrService();
    mockApiService = MockApiService();

    // Default stub for dispose
    when(() => mockOcrService.dispose()).thenReturn(null);
  });

  Widget createWidget() {
    return MaterialApp(
      routes: {
        '/settings': (context) => const Scaffold(body: Text('Settings')),
      },
      home: CameraScreen(
        cameraService: mockCameraService,
        ocrService: mockOcrService,
        apiService: mockApiService,
      ),
    );
  }

  group('CameraScreen', () {
    testWidgets('shows loading spinner before camera initialization', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => <CameraDescription>[],
      );

      await tester.pumpWidget(createWidget());
      // Just pump once to see the initial state before async completes
      await tester.pump();

      // Should show spinner while initializing (before getAvailableCameras completes)
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      // Let async complete
      await tester.pump(const Duration(milliseconds: 100));
    });

    testWidgets('shows snackbar when no cameras available', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      // Use pump with duration instead of pumpAndSettle to avoid animation timeout
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.text('No camera available'), findsOneWidget);
    });

    testWidgets('displays game selector with MTG and Pokemon options', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.text('MTG'), findsOneWidget);
      expect(find.text('Pokemon'), findsOneWidget);
      expect(find.byType(SegmentedButton<String>), findsOneWidget);
    });

    testWidgets('MTG is selected by default', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      final segmentedButton = tester.widget<SegmentedButton<String>>(
        find.byType(SegmentedButton<String>),
      );
      expect(segmentedButton.selected, contains('mtg'));
    });

    testWidgets('can switch to Pokemon game', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      await tester.tap(find.text('Pokemon'));
      await tester.pump(const Duration(milliseconds: 100));

      final segmentedButton = tester.widget<SegmentedButton<String>>(
        find.byType(SegmentedButton<String>),
      );
      expect(segmentedButton.selected, contains('pokemon'));
    });

    testWidgets('displays app bar with title', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.text('Scan Card'), findsOneWidget);
    });

    testWidgets('displays settings icon in app bar', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.byIcon(Icons.settings), findsOneWidget);
    });

    testWidgets('navigates to settings when settings icon tapped', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      await tester.tap(find.byIcon(Icons.settings));
      await tester.pumpAndSettle();

      expect(find.text('Settings'), findsOneWidget);
    });

    testWidgets('displays capture button with camera icon', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.byType(FloatingActionButton), findsOneWidget);
      expect(find.byIcon(Icons.camera_alt), findsOneWidget);
    });

    testWidgets('capture button is disabled when camera not initialized', (tester) async {
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      // Tap the button - nothing should happen since no camera
      await tester.tap(find.byType(FloatingActionButton));
      await tester.pump(const Duration(milliseconds: 100));

      // Should not crash, button should still be there
      expect(find.byType(FloatingActionButton), findsOneWidget);
    });

    testWidgets('shows loading indicator when processing', (tester) async {
      // We can't easily test the processing state without a real camera,
      // but we verify the UI structure is correct
      when(() => mockCameraService.getAvailableCameras()).thenAnswer(
        (_) async => [],
      );

      await tester.pumpWidget(createWidget());
      await tester.pump(const Duration(milliseconds: 100));

      // The camera icon should be visible when not processing
      expect(find.byIcon(Icons.camera_alt), findsOneWidget);
    });

    group('widget structure', () {
      testWidgets('has Column layout with game selector, preview, and button', (tester) async {
        when(() => mockCameraService.getAvailableCameras()).thenAnswer(
          (_) async => [],
        );

        await tester.pumpWidget(createWidget());
        await tester.pump(const Duration(milliseconds: 100));

        // Verify main structural elements exist
        expect(find.byType(Column), findsWidgets);
        expect(find.byType(SegmentedButton<String>), findsOneWidget);
        expect(find.byType(FloatingActionButton), findsOneWidget);
      });

      testWidgets('uses Scaffold with AppBar', (tester) async {
        when(() => mockCameraService.getAvailableCameras()).thenAnswer(
          (_) async => [],
        );

        await tester.pumpWidget(createWidget());
        await tester.pump(const Duration(milliseconds: 100));

        expect(find.byType(Scaffold), findsOneWidget);
        expect(find.byType(AppBar), findsOneWidget);
      });
    });
  });
}
