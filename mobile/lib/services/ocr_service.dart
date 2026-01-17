import 'dart:io';
import 'package:google_mlkit_text_recognition/google_mlkit_text_recognition.dart';

/// Abstraction layer for OCR operations to enable testing
class OcrService {
  final TextRecognizer _recognizer;

  OcrService({TextRecognizer? recognizer})
      : _recognizer = recognizer ?? TextRecognizer();

  /// Extract text lines from an image file
  /// Throws [OcrException] if the image cannot be processed
  Future<List<String>> extractTextFromImage(String imagePath) async {
    // Validate file exists
    final file = File(imagePath);
    if (!await file.exists()) {
      throw OcrException('Image file not found: $imagePath');
    }

    try {
      final inputImage = InputImage.fromFilePath(imagePath);
      final recognizedText = await _recognizer.processImage(inputImage);

      return recognizedText.blocks
          .expand((block) => block.lines)
          .map((line) => line.text)
          .toList();
    } catch (e) {
      if (e is OcrException) rethrow;
      throw OcrException('Failed to process image: $e');
    }
  }

  /// Clean up resources
  void dispose() {
    _recognizer.close();
  }
}

/// Exception thrown when OCR processing fails
class OcrException implements Exception {
  final String message;
  OcrException(this.message);

  @override
  String toString() => 'OcrException: $message';
}
