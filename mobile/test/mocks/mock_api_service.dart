import 'package:mocktail/mocktail.dart';
import 'package:mobile/models/card.dart';
import 'package:mobile/models/collection_item.dart';
import 'package:mobile/services/api_service.dart';

/// Mock ApiService for testing
class MockApiService extends Mock implements ApiService {}

/// Extension to provide common stub setups
extension MockApiServiceExtension on MockApiService {
  /// Stubs getServerUrl to return the given URL
  void stubGetServerUrl(String url) {
    when(() => getServerUrl()).thenAnswer((_) async => url);
  }

  /// Stubs setServerUrl to complete successfully
  void stubSetServerUrl() {
    when(() => setServerUrl(any())).thenAnswer((_) async {});
  }

  /// Stubs searchCards to return the given result
  void stubSearchCards(CardSearchResult result) {
    when(
      () => searchCards(any(), any(), setIDs: any(named: 'setIDs')),
    ).thenAnswer((_) async => result);
  }

  /// Stubs searchCards to throw an exception
  void stubSearchCardsError(String message) {
    when(
      () => searchCards(any(), any(), setIDs: any(named: 'setIDs')),
    ).thenThrow(Exception(message));
  }

  /// Stubs identifyCard to return the given result
  void stubIdentifyCard(ScanResult result) {
    when(() => identifyCard(any(), any())).thenAnswer((_) async => result);
  }

  /// Stubs identifyCard to throw an exception
  void stubIdentifyCardError(String message) {
    when(() => identifyCard(any(), any())).thenThrow(Exception(message));
  }

  /// Stubs identifyCard to throw a timeout exception
  void stubIdentifyCardTimeout() {
    when(
      () => identifyCard(any(), any()),
    ).thenThrow(Exception('Request timed out'));
  }

  /// Stubs addToCollection to complete successfully
  void stubAddToCollection([CollectionItem? item]) {
    final defaultItem =
        item ??
        CollectionItem(
          id: 1,
          cardId: 'test-card-id',
          card: CardModel(
            id: 'test-card-id',
            game: 'pokemon',
            name: 'Test Card',
          ),
          quantity: 1,
          condition: 'NM',
          printing: PrintingType.normal,
          addedAt: DateTime.now(),
        );
    when(
      () => addToCollection(
        any(),
        quantity: any(named: 'quantity'),
        condition: any(named: 'condition'),
        printing: any(named: 'printing'),
        scannedImageBytes: any(named: 'scannedImageBytes'),
        language: any(named: 'language'),
        ocrText: any(named: 'ocrText'),
      ),
    ).thenAnswer((_) async => defaultItem);
  }

  /// Stubs addToCollection to throw an exception
  void stubAddToCollectionError(String message) {
    when(
      () => addToCollection(
        any(),
        quantity: any(named: 'quantity'),
        condition: any(named: 'condition'),
        printing: any(named: 'printing'),
        scannedImageBytes: any(named: 'scannedImageBytes'),
        language: any(named: 'language'),
        ocrText: any(named: 'ocrText'),
      ),
    ).thenThrow(Exception(message));
  }

  /// Stubs identifyCardFromImage to return the given result
  void stubIdentifyCardFromImage(ScanResult result) {
    when(
      () => identifyCardFromImage(any(), any()),
    ).thenAnswer((_) async => result);
  }

  /// Stubs identifyCardFromImage to throw an exception
  void stubIdentifyCardFromImageError(String message) {
    when(
      () => identifyCardFromImage(any(), any()),
    ).thenThrow(Exception(message));
  }

  /// Stubs isServerOCRAvailable to return the given value
  void stubIsServerOCRAvailable(bool available) {
    when(() => isServerOCRAvailable()).thenAnswer((_) async => available);
  }
}
