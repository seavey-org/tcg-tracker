import 'package:flutter_test/flutter_test.dart';
import 'package:google_mlkit_text_recognition/google_mlkit_text_recognition.dart';
import 'package:mocktail/mocktail.dart';
import 'package:mobile/services/ocr_service.dart';
import 'dart:io';

class MockTextRecognizer extends Mock implements TextRecognizer {}
class MockRecognizedText extends Mock implements RecognizedText {}
class MockTextBlock extends Mock implements TextBlock {}
class MockTextLine extends Mock implements TextLine {}
class MockInputImage extends Mock implements InputImage {}

void main() {
  late OcrService ocrService;
  late MockTextRecognizer mockRecognizer;

  setUpAll(() {
    registerFallbackValue(InputImage.fromFilePath('test_assets/sample_card.jpg'));
  });

  setUp(() {
    mockRecognizer = MockTextRecognizer();
    ocrService = OcrService(recognizer: mockRecognizer);
  });

  group('OcrService', () {
    test('extractTextFromImage returns lines of text on success', () async {
      // Create a temporary file to satisfy the existence check
      final file = File('test_image.jpg');
      await file.writeAsBytes([0]);

      final mockRecognizedText = MockRecognizedText();
      final mockBlock = MockTextBlock();
      final mockLine1 = MockTextLine();
      final mockLine2 = MockTextLine();

      when(() => mockLine1.text).thenReturn('Charizard VMAX');
      when(() => mockLine2.text).thenReturn('330 HP');
      when(() => mockBlock.lines).thenReturn([mockLine1, mockLine2]);
      when(() => mockRecognizedText.blocks).thenReturn([mockBlock]);
      
      when(() => mockRecognizer.processImage(any()))
          .thenAnswer((_) async => mockRecognizedText);

      final result = await ocrService.extractTextFromImage('test_image.jpg');

      expect(result, ['Charizard VMAX', '330 HP']);
      
      // Cleanup
      await file.delete();
    });

    test('extractTextFromImage throws OcrException if file does not exist', () async {
      expect(
        () => ocrService.extractTextFromImage('non_existent.jpg'),
        throwsA(isA<OcrException>().having(
          (e) => e.toString(),
          'message',
          contains('Image file not found'),
        )),
      );
    });

    test('extractTextFromImage wraps generic exceptions in OcrException', () async {
      final file = File('test_image_error.jpg');
      await file.writeAsBytes([0]);

      when(() => mockRecognizer.processImage(any()))
          .thenThrow(Exception('Native OCR error'));

      expect(
        () => ocrService.extractTextFromImage('test_image_error.jpg'),
        throwsA(isA<OcrException>().having(
          (e) => e.toString(),
          'message',
          contains('Failed to process image'),
        )),
      );

      await file.delete();
    });

    test('dispose calls close on recognizer', () {
      when(() => mockRecognizer.close()).thenAnswer((_) async => {});
      
      ocrService.dispose();
      
      verify(() => mockRecognizer.close()).called(1);
    });
  });
}
