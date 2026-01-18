import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:network_image_mock/network_image_mock.dart';
import 'package:mobile/models/card.dart';
import 'package:mobile/screens/scan_result_screen.dart';
import '../../fixtures/card_fixtures.dart';
import '../../fixtures/scan_fixtures.dart';
import '../../mocks/mock_api_service.dart';

void main() {
  late MockApiService mockApiService;

  setUp(() {
    mockApiService = MockApiService();
  });

  Widget createWidget({
    List<CardModel>? cards,
    String searchQuery = 'Test Query',
    ScanMetadata? scanMetadata,
    SetIconResult? setIcon,
  }) {
    return MaterialApp(
      home: ScanResultScreen(
        cards: cards ?? [],
        searchQuery: searchQuery,
        game: 'pokemon',
        scanMetadata: scanMetadata,
        setIcon: setIcon,
        apiService: mockApiService,
      ),
    );
  }

  group('ScanResultScreen', () {
    group('empty state', () {
      testWidgets('shows "No cards found" when cards list is empty', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(createWidget(cards: []));
          await tester.pumpAndSettle();

          expect(find.text('No cards found'), findsOneWidget);
        });
      });
    });

    group('card list display', () {
      testWidgets('displays card names', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanMetadata(cardName: 'Pikachu'),
              setIcon: SetIconResult(
                bestSetId: 'base1',
                confidence: 0.2,
                lowConfidence: true,
                candidates: [SetIconCandidate(setId: 'base1', score: 0.2)],
              ),
            ),
          );
          await tester.pumpAndSettle();

          expect(find.text('Charizard VMAX'), findsOneWidget);
          expect(find.text('Lightning Bolt'), findsNothing);
          expect(find.text('Test Card'), findsNothing);
        });
      });

      testWidgets('displays card sets and prices', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(cards: [CardFixtures.completeCard]),
          );
          await tester.pumpAndSettle();

          expect(find.textContaining('Vivid Voltage'), findsOneWidget);
          expect(find.textContaining('\$125.50'), findsOneWidget);
        });
      });

      testWidgets('displays app bar with search query', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              searchQuery: 'Charizard',
            ),
          );
          await tester.pumpAndSettle();

          expect(find.text('Results for "Charizard"'), findsOneWidget);
        });
      });

      testWidgets('shows add button for each card', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard, CardFixtures.mtgCard],
            ),
          );
          await tester.pumpAndSettle();

          expect(find.byIcon(Icons.add_circle), findsNWidgets(2));
        });
      });
    });

    group('metadata card', () {
      testWidgets('shows metadata card when confidence > 0', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.completeScanMetadata,
            ),
          );
          await tester.pumpAndSettle();

          expect(find.text('Scan Detection'), findsOneWidget);
          expect(find.textContaining('85% confidence'), findsOneWidget);
        });
      });

      testWidgets('hides metadata card when confidence is 0', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.minimalScanMetadata,
            ),
          );
          await tester.pumpAndSettle();

          expect(find.text('Scan Detection'), findsNothing);
        });
      });

      testWidgets('hides metadata card when scanMetadata is null', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: null,
            ),
          );
          await tester.pumpAndSettle();

          expect(find.text('Scan Detection'), findsNothing);
        });
      });

      testWidgets('shows foil indicator chips when foil indicators present', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.foilMetadata,
            ),
          );
          await tester.pumpAndSettle();

          expect(find.text('HOLO'), findsOneWidget);
          expect(find.text('REVERSE'), findsOneWidget);
        });
      });

      testWidgets('shows green confidence badge for high confidence', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.highConfidenceMetadata,
            ),
          );
          await tester.pumpAndSettle();

          expect(find.textContaining('75% confidence'), findsOneWidget);
        });
      });

      testWidgets('shows detection summary', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.completeScanMetadata,
            ),
          );
          await tester.pumpAndSettle();

          expect(find.textContaining('Charizard VMAX'), findsWidgets);
        });
      });
    });

    group('add dialog', () {
      testWidgets('tap on card opens add dialog', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(cards: [CardFixtures.completeCard]),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();

          // Now goes through confirmation screen first
          expect(find.text('Confirm Card'), findsOneWidget);
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Add Charizard VMAX'), findsOneWidget);
          expect(find.text('Add to Collection'), findsOneWidget);
        });
      });

      testWidgets('tap on add button opens dialog', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(cards: [CardFixtures.completeCard]),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.byIcon(Icons.add_circle));
          await tester.pumpAndSettle();

          expect(find.text('Confirm Card'), findsOneWidget);
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Add Charizard VMAX'), findsOneWidget);
        });
      });

      testWidgets('add dialog pre-fills foil from scanMetadata', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.foilMetadata,
            ),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          // Find the first Switch (Foil) in SwitchListTile and verify it's on
          // Note: There are now two Switches (Foil and First Edition)
          final switchWidgets = tester
              .widgetList<Switch>(find.byType(Switch))
              .toList();
          expect(switchWidgets.length, 2); // Foil and First Edition
          expect(switchWidgets[0].value, isTrue); // First switch is Foil
        });
      });

      testWidgets('add dialog shows "Detected" label when foil detected', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.foilMetadata,
            ),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Detected'), findsOneWidget);
        });
      });

      testWidgets('add dialog has quantity controls', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(cards: [CardFixtures.completeCard]),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
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
            createWidget(cards: [CardFixtures.completeCard]),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();

          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          expect(find.text('Condition:'), findsOneWidget);
          // Dropdown now shows condition code with description
          expect(find.text('NM - Near Mint'), findsOneWidget);
        });
      });
    });

    group('confirm flow', () {
      testWidgets('retake from confirm returns to previous route', (
        tester,
      ) async {
        await mockNetworkImagesFor(() async {
          // Wrap results so we can verify popping back out of it.
          await tester.pumpWidget(
            MaterialApp(
              home: Builder(
                builder: (context) => Scaffold(
                  body: TextButton(
                    onPressed: () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => ScanResultScreen(
                          cards: [CardFixtures.completeCard],
                          searchQuery: 'Test Query',
                          game: 'pokemon',
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
          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();
          expect(find.text('Confirm Card'), findsOneWidget);

          // Retake should pop confirm and results, returning to opener.
          await tester.tap(find.text('Retake'));
          await tester.pumpAndSettle();

          expect(find.text('Open'), findsOneWidget);
          expect(find.byType(ScanResultScreen), findsNothing);
        });
      });

      testWidgets('can browse likely prints and pick a different card', (
        tester,
      ) async {
        // Browse uses scanMetadata.cardName, not the content entered in the search box.
        mockApiService.stubSearchCards(
          CardSearchResult(
            cards: [CardFixtures.mtgCard],
            totalCount: 1,
            hasMore: false,
          ),
        );

        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanMetadata(cardName: 'Pikachu'),
              setIcon: SetIconResult(
                bestSetId: 'base1',
                confidence: 0.2,
                lowConfidence: true,
                candidates: [SetIconCandidate(setId: 'base1', score: 0.2)],
              ),
            ),
          );
          await tester.pumpAndSettle();

          // Open confirm screen
          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();
          expect(find.text('Confirm Card'), findsOneWidget);

          // Browse likely prints (inline list) and pick a different card
          await tester.tap(find.text('Browse likely prints'));
          await tester.pumpAndSettle();

          final scrollable = find.byType(Scrollable).last;
          await tester.scrollUntilVisible(
            find.widgetWithText(ListTile, 'Lightning Bolt').first,
            200,
            scrollable: scrollable,
          );
          await tester.tap(
            find.widgetWithText(ListTile, 'Lightning Bolt').first,
          );
          await tester.pumpAndSettle();

          // Now add sheet should be for selected card
          expect(find.text('Add Lightning Bolt'), findsOneWidget);
        });
      });

      testWidgets(
        'browse likely prints shows error when scanMetadata is null',
        (tester) async {
          await mockNetworkImagesFor(() async {
            await tester.pumpWidget(
              createWidget(
                cards: [CardFixtures.completeCard],
                scanMetadata: null,
                setIcon: SetIconResult(
                  bestSetId: 'base1',
                  confidence: 0.2,
                  lowConfidence: true,
                  candidates: [SetIconCandidate(setId: 'base1', score: 0.2)],
                ),
              ),
            );
            await tester.pumpAndSettle();

            await tester.tap(find.text('Charizard VMAX'));
            await tester.pumpAndSettle();
            expect(find.text('Confirm Card'), findsOneWidget);

            await tester.tap(find.text('Browse likely prints'));
            await tester.pumpAndSettle();

            expect(
              find.text(
                'No detected card name available to browse. Retake the photo.',
              ),
              findsOneWidget,
            );
          });
        },
      );

      testWidgets(
        'browse likely prints shows error when detected name is empty',
        (tester) async {
          await mockNetworkImagesFor(() async {
            await tester.pumpWidget(
              createWidget(
                cards: [CardFixtures.completeCard],
                scanMetadata: ScanMetadata(cardName: ''),
                setIcon: SetIconResult(
                  bestSetId: 'base1',
                  confidence: 0.2,
                  lowConfidence: true,
                  candidates: [SetIconCandidate(setId: 'base1', score: 0.2)],
                ),
              ),
            );
            await tester.pumpAndSettle();

            await tester.tap(find.text('Charizard VMAX'));
            await tester.pumpAndSettle();
            expect(find.text('Confirm Card'), findsOneWidget);

            await tester.tap(find.text('Browse likely prints'));
            await tester.pumpAndSettle();

            expect(
              find.text(
                'No detected card name available to browse. Retake the photo.',
              ),
              findsOneWidget,
            );
          });
        },
      );
    });

    group('add to collection', () {
      testWidgets('shows success snackbar on successful add', (tester) async {
        mockApiService.stubAddToCollection();

        await mockNetworkImagesFor(() async {
          // Wrap in a parent screen so Navigator.pop has somewhere to go
          await tester.pumpWidget(
            MaterialApp(
              home: Builder(
                builder: (context) => Scaffold(
                  body: TextButton(
                    onPressed: () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => ScanResultScreen(
                          cards: [CardFixtures.completeCard],
                          searchQuery: 'Test Query',
                          game: 'pokemon',
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

          // Navigate to the scan result screen
          await tester.tap(find.text('Open'));
          await tester.pumpAndSettle();

          // Open confirmation first
          await tester.tap(find.text('Charizard VMAX'));
          await tester.pumpAndSettle();
          await tester.tap(find.text('Yes, this is correct'));
          await tester.pumpAndSettle();

          // Tap add to collection
          await tester.tap(find.text('Add to Collection'));
          // Use pump to catch the snackbar before navigation completes
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
            foil: any(named: 'foil'),
          ),
        ).thenThrow(Exception('Network error'));

        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(cards: [CardFixtures.completeCard]),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
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
            createWidget(
              cards: [CardFixtures.completeCard],
              scanMetadata: ScanFixtures.foilMetadata,
            ),
          );
          await tester.pumpAndSettle();

          await tester.tap(find.text('Charizard VMAX'));
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
              foil: true,
            ),
          ).called(1);
        });
      });
    });

    group('image handling', () {
      testWidgets('shows image icon when imageUrl is null', (tester) async {
        await mockNetworkImagesFor(() async {
          await tester.pumpWidget(
            createWidget(cards: [CardFixtures.nullOptionalsCard]),
          );
          await tester.pumpAndSettle();

          expect(find.byIcon(Icons.image), findsOneWidget);
        });
      });
    });
  });
}
