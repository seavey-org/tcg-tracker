import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:network_image_mock/network_image_mock.dart';
import 'package:mobile/models/collection_item.dart' show PrintingType;
import 'package:mobile/models/gemini_scan_result.dart';
import 'package:mobile/screens/scan_result_screen.dart';
import '../../fixtures/card_fixtures.dart';
import '../../fixtures/scan_fixtures.dart';
import '../../mocks/mock_api_service.dart';

void main() {
  late MockApiService mockApiService;

  setUpAll(() {
    registerFallbackValue(PrintingType.normal);
  });

  setUp(() {
    mockApiService = MockApiService();
  });

  Widget createWidget({
    required GeminiScanResult geminiResult,
    List<int>? scannedImageBytes,
  }) {
    return MaterialApp(
      home: ScanResultScreen(
        geminiResult: geminiResult,
        apiService: mockApiService,
        scannedImageBytes: scannedImageBytes,
      ),
    );
  }

  group('ScanResultScreen', () {
    group('empty state', () {
      testWidgets('shows "No cards found" when cards list is empty', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.noMatchGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.text('No cards found'), findsOneWidget);
        });
      });
    });

    group('card list display', () {
      testWidgets('displays card names', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.text('Charizard VMAX'), findsWidgets);
        });
      });

      testWidgets('displays card sets and prices', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.textContaining('Vivid Voltage'), findsWidgets);
          expect(find.textContaining('\$125.50'), findsWidgets);
        });
      });

      testWidgets('displays app bar with card name', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.text('Results for "Charizard VMAX"'), findsOneWidget);
        });
      });

      testWidgets('shows add button for each card', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.multiCandidateGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.byIcon(Icons.add_circle), findsNWidgets(2));
        });
      });
    });

    group('Gemini info card', () {
      testWidgets('shows Gemini info card with confidence', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.text('Gemini Identification'), findsOneWidget);
          expect(find.textContaining('85%'), findsOneWidget);
        });
      });

      testWidgets('shows reasoning in expansion tile', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.text('How it was identified'), findsOneWidget);
        });
      });

      testWidgets('shows language badge for non-English cards', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.japaneseGeminiResult),
          );
          await tester.pumpAndSettle();

          expect(find.text('Japanese'), findsOneWidget);
        });
      });
    });

    group('add dialog', () {
      testWidgets('tap on card opens confirm then add dialog', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();

          // Confirm screen first
          expect(find.text('Confirm Card'), findsOneWidget);
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Add Charizard VMAX'), findsOneWidget);
          expect(find.text('Add to Collection'), findsOneWidget);
        });
      });

      testWidgets('add dialog has quantity controls', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Quantity:'), findsOneWidget);
          expect(find.text('1'), findsOneWidget);
          expect(find.byIcon(Icons.add), findsOneWidget);
          expect(find.byIcon(Icons.remove), findsOneWidget);
        });
      });

      testWidgets('add dialog has condition dropdown', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Condition:'), findsOneWidget);
          expect(find.text('NM - Near Mint'), findsOneWidget);
        });
      });

      testWidgets('add dialog has language dropdown', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Language:'), findsOneWidget);
          expect(find.text('English'), findsOneWidget);
        });
      });

      testWidgets('add dialog pre-fills language from Gemini detection', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.japaneseGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          // Should show Japanese pre-selected in dropdown
          expect(find.text('Japanese'), findsWidgets);
          // Should show "Detected" label
          expect(find.text('Detected'), findsOneWidget);
        });
      });
    });

    group('confirm flow', () {
      testWidgets('retake from confirm returns to previous route', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            MaterialApp(
              home: Builder(
                builder: (context) => Scaffold(
                  body: TextButton(
                    onPressed: () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => ScanResultScreen(
                          geminiResult: ScanFixtures.completeGeminiResult,
                          apiService: mockApiService,
                        ),
                      ),
                    ),
                    child: const Text('Open'),
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Open'));
          await tester.pumpAndSettle();
          expect(find.byType(ScanResultScreen), findsOneWidget);

          // Open confirm screen
          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();
          expect(find.text('Confirm Card'), findsOneWidget);

          // Retake should pop confirm and results
          await tester.tap(find.text('Retake'));
          await tester.pumpAndSettle();

          expect(find.text('Open'), findsOneWidget);
          expect(find.byType(ScanResultScreen), findsNothing);
        });
      });
    });

    group('add to collection', () {
      testWidgets('shows success snackbar on successful add', (tester) async {
        mockApiService.stubAddToCollection();

        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            MaterialApp(
              home: Builder(
                builder: (context) => Scaffold(
                  body: TextButton(
                    onPressed: () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => ScanResultScreen(
                          geminiResult: ScanFixtures.completeGeminiResult,
                          apiService: mockApiService,
                        ),
                      ),
                    ),
                    child: const Text('Open'),
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Open'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Add to Collection'));
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 100));

          expect(
            find.text('Added Charizard VMAX to collection!'),
            findsOneWidget,
          );
        });
      });

      testWidgets('shows error snackbar on failed add', (tester) async {
        when(
          () => mockApiService.addToCollection(
            any(),
            quantity: any(named: 'quantity'),
            condition: any(named: 'condition'),
            printing: any(named: 'printing'),
          ),
        ).thenThrow(Exception('Network error'));

        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Add to Collection'));
          await tester.pumpAndSettle();

          expect(find.textContaining('Error:'), findsOneWidget);
        });
      });

      testWidgets('calls addToCollection with correct parameters', (
        tester,
      ) async {
        mockApiService.stubAddToCollection();

        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.completeGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Add to Collection'));
          await tester.pumpAndSettle();

          verify(
            () => mockApiService.addToCollection(
              'swsh4-025',
              quantity: 1,
              condition: 'NM',
              printing: PrintingType.normal,
              scannedImageBytes: any(named: 'scannedImageBytes'),
              language: 'English',
              ocrText: any(named: 'ocrText'),
            ),
          ).called(1);
        });
      });

      testWidgets('passes detected Japanese language when adding', (
        tester,
      ) async {
        mockApiService.stubAddToCollection();

        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.japaneseGeminiResult),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX').first);
          await tester.pumpAndSettle();
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Add to Collection'));
          await tester.pumpAndSettle();

          verify(
            () => mockApiService.addToCollection(
              'swsh4-025',
              quantity: 1,
              condition: 'NM',
              printing: PrintingType.normal,
              scannedImageBytes: any(named: 'scannedImageBytes'),
              language: 'Japanese',
              ocrText: any(named: 'ocrText'),
            ),
          ).called(1);
        });
      });
    });

    group('image handling', () {
      testWidgets('shows image icon when imageUrl is null', (tester) async {
        await mockNetworkImagesFor(() async {
          // Create a result with a card that has no image
          final result = GeminiScanResult(
            cardId: 'test-id',
            cardName: 'Test Card',
            canonicalNameEN: 'Test Card',
            setCode: 'test',
            setName: 'Test Set',
            cardNumber: '001',
            game: 'pokemon',
            observedLanguage: 'English',
            confidence: 0.8,
            reasoning: 'Test',
            turnsUsed: 1,
            cards: [CardFixtures.nullOptionalsCard],
          );

          await tester.pumpWidget(createWidget(geminiResult: result));
          await tester.pumpAndSettle();

          expect(find.byIcon(Icons.image), findsOneWidget);
        });
      });
    });

    group('MTG 2-phase selection', () {
      testWidgets('shows set selection for MTG with multiple sets', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(geminiResult: ScanFixtures.mtgMultiSetGeminiResult),
          );
          await tester.pumpAndSettle();

          // Should show set selection UI
          expect(find.text('3 sets found'), findsOneWidget);
        });
      });
    });
  });
}
