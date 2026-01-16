import 'dart:io';
import 'package:flutter/material.dart';
import 'package:camera/camera.dart';
import 'package:google_mlkit_text_recognition/google_mlkit_text_recognition.dart';
import '../services/api_service.dart';
import 'scan_result_screen.dart';

class CameraScreen extends StatefulWidget {
  const CameraScreen({super.key});

  @override
  State<CameraScreen> createState() => _CameraScreenState();
}

class _CameraScreenState extends State<CameraScreen> {
  CameraController? _controller;
  List<CameraDescription>? _cameras;
  bool _isInitialized = false;
  bool _isProcessing = false;
  String _selectedGame = 'mtg';
  final TextRecognizer _textRecognizer = TextRecognizer();
  final ApiService _apiService = ApiService();

  @override
  void initState() {
    super.initState();
    _initializeCamera();
  }

  Future<void> _initializeCamera() async {
    _cameras = await availableCameras();
    if (_cameras == null || _cameras!.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('No camera available')),
        );
      }
      return;
    }

    _controller = CameraController(
      _cameras!.first,
      ResolutionPreset.high,
      enableAudio: false,
    );

    await _controller!.initialize();
    if (mounted) {
      setState(() => _isInitialized = true);
    }
  }

  Future<void> _captureAndProcess() async {
    if (_controller == null || !_controller!.value.isInitialized || _isProcessing) {
      return;
    }

    setState(() => _isProcessing = true);

    try {
      final image = await _controller!.takePicture();
      final inputImage = InputImage.fromFilePath(image.path);
      final recognizedText = await _textRecognizer.processImage(inputImage);

      // Extract text from image
      final lines = recognizedText.blocks
          .expand((block) => block.lines)
          .map((line) => line.text)
          .toList();

      if (lines.isEmpty) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('No text detected in image')),
          );
        }
        return;
      }

      // Send full OCR text to server for parsing (includes card number, set info)
      final fullText = lines.join('\n');

      // Search for matching cards using identify endpoint
      final cards = await _apiService.identifyCard(fullText, _selectedGame);

      if (!mounted) return;

      // Clean up temp image
      await File(image.path).delete();

      if (cards.isEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('No cards found')),
        );
        return;
      }

      // Navigate to results - best match should be first
      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (context) => ScanResultScreen(
            cards: cards,
            searchQuery: 'Scanned Card',
          ),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error: ${e.toString()}')),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _isProcessing = false);
      }
    }
  }

  @override
  void dispose() {
    _controller?.dispose();
    _textRecognizer.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Scan Card'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
        actions: [
          IconButton(
            icon: const Icon(Icons.settings),
            onPressed: () => Navigator.pushNamed(context, '/settings'),
          ),
        ],
      ),
      body: Column(
        children: [
          // Game selector
          Padding(
            padding: const EdgeInsets.all(8.0),
            child: SegmentedButton<String>(
              segments: const [
                ButtonSegment(value: 'mtg', label: Text('MTG')),
                ButtonSegment(value: 'pokemon', label: Text('Pokemon')),
              ],
              selected: {_selectedGame},
              onSelectionChanged: (selection) {
                setState(() => _selectedGame = selection.first);
              },
            ),
          ),
          // Camera preview
          Expanded(
            child: _isInitialized && _controller != null
                ? ClipRRect(
                    borderRadius: BorderRadius.circular(12),
                    child: CameraPreview(_controller!),
                  )
                : const Center(child: CircularProgressIndicator()),
          ),
          // Capture button
          Padding(
            padding: const EdgeInsets.all(24.0),
            child: FloatingActionButton.large(
              onPressed: _isProcessing ? null : _captureAndProcess,
              child: _isProcessing
                  ? const CircularProgressIndicator(color: Colors.white)
                  : const Icon(Icons.camera_alt),
            ),
          ),
        ],
      ),
    );
  }
}
