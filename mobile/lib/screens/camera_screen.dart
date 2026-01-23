import 'dart:async';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:camera/camera.dart';
import 'package:permission_handler/permission_handler.dart';
import '../models/card.dart';
import '../services/api_service.dart';
import '../services/camera_service.dart';
import '../services/image_analysis_service.dart';
import '../services/ocr_service.dart';
import 'scan_result_screen.dart';

/// Card aspect ratio (width / height) - standard for Pokemon and MTG cards (2.5" x 3.5")
const double cardAspectRatio = 0.714;

/// Camera screen for scanning trading cards.
///
/// Uses server-side OCR when available for better accuracy, falling back
/// to client-side ML Kit OCR when the server is unavailable.
class CameraScreen extends StatefulWidget {
  final CameraService? cameraService;
  final OcrService? ocrService;
  final ApiService? apiService;

  const CameraScreen({
    super.key,
    this.cameraService,
    this.ocrService,
    this.apiService,
  });

  @override
  State<CameraScreen> createState() => _CameraScreenState();
}

class _CameraScreenState extends State<CameraScreen> {
  CameraController? _controller;
  List<CameraDescription>? _cameras;
  bool _isInitialized = false;
  bool _isProcessing = false;
  String _selectedGame = 'mtg';
  late final CameraService _cameraService;
  late final OcrService _ocrService;
  late final ApiService _apiService;
  late final ImageAnalysisService _imageAnalysisService;
  bool? _serverOCRAvailable;

  // Quality feedback state
  String _qualityFeedback = 'Initializing...';
  bool _isQualityAcceptable = false;
  Timer? _qualityCheckTimer;

  // Mutex flag to prevent concurrent takePicture() calls
  // The camera package throws errors if you capture while another capture is in progress
  bool _isTakingPicture = false;

  // Avoid spamming the user with repeated warnings.
  bool _shownJapaneseFallbackWarning = false;

  @override
  void initState() {
    super.initState();
    _cameraService = widget.cameraService ?? CameraService();
    _ocrService = widget.ocrService ?? OcrService();
    _apiService = widget.apiService ?? ApiService();
    _imageAnalysisService = ImageAnalysisService();
    _initializeCamera();
    _checkServerOCR();
  }

  Future<void> _checkServerOCR() async {
    try {
      final available = await _apiService.isServerOCRAvailable();
      if (mounted) {
        setState(() => _serverOCRAvailable = available);
      }
    } catch (e) {
      // Server OCR check failed, assume unavailable
      if (mounted) {
        setState(() => _serverOCRAvailable = false);
      }
    }
  }

  Future<void> _initializeCamera() async {
    // Check camera permission first
    final status = await Permission.camera.status;
    if (!status.isGranted) {
      final result = await Permission.camera.request();
      if (!result.isGranted) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: const Text('Camera permission denied'),
              action: SnackBarAction(
                label: 'Settings',
                onPressed: () => openAppSettings(),
              ),
            ),
          );
        }
        return;
      }
    }

    _cameras = await _cameraService.getAvailableCameras();
    if (_cameras == null || _cameras!.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('No camera available')));
      }
      return;
    }

    _controller = _cameraService.createController(
      _cameras!.first,
      resolutionPreset: ResolutionPreset.high,
      enableAudio: false,
    );

    try {
      await _controller!.initialize();
      if (mounted) {
        setState(() {
          _isInitialized = true;
          _qualityFeedback = 'Position card in frame';
        });
        _startQualityChecking();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to initialize camera: $e')),
        );
      }
    }
  }

  void _startQualityChecking() {
    // Check quality every 500ms from camera preview
    _qualityCheckTimer = Timer.periodic(
      const Duration(milliseconds: 500),
      (_) => _checkQuality(),
    );
  }

  Future<void> _checkQuality() async {
    if (_controller == null ||
        !_controller!.value.isInitialized ||
        _isProcessing ||
        _isTakingPicture) {
      return;
    }

    // Set flag before any async work to prevent race conditions
    _isTakingPicture = true;

    try {
      // Take a preview image for quality analysis
      final image = await _controller!.takePicture();
      final bytes = await File(image.path).readAsBytes();

      final quality = _imageAnalysisService.checkImageQuality(bytes);

      // Clean up temp file
      await File(image.path).delete();

      if (mounted) {
        setState(() {
          _qualityFeedback = quality.feedbackMessage;
          _isQualityAcceptable = quality.isAcceptable;
        });
      }
    } catch (e) {
      // Ignore errors during quality checking
    } finally {
      _isTakingPicture = false;
    }
  }

  Future<void> _captureAndProcess() async {
    if (_controller == null ||
        !_controller!.value.isInitialized ||
        _isProcessing ||
        _isTakingPicture) {
      return;
    }

    setState(() => _isProcessing = true);

    // Set flag before capture to prevent quality check from interfering
    _isTakingPicture = true;

    XFile image;
    try {
      image = await _controller!.takePicture();
    } catch (e) {
      // Capture failed (could be concurrent capture attempt or hardware issue)
      _isTakingPicture = false;
      if (mounted) {
        setState(() => _isProcessing = false);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Failed to capture image: $e')));
      }
      return;
    }
    // Release capture lock immediately after takePicture completes
    // This allows quality checking to resume while we process the image
    _isTakingPicture = false;

    try {
      final imageBytes = await File(image.path).readAsBytes();
      final imageBytesAsList = imageBytes.toList();

      ScanResult? scanResult;
      bool usedServerOCR = false;

      // Prefer server-side OCR when available (better parsing and accuracy)
      if (_serverOCRAvailable == true) {
        try {
          scanResult = await _apiService.identifyCardFromImage(
            imageBytesAsList,
            _selectedGame,
          );
          usedServerOCR = true;
        } catch (e) {
          // Server OCR failed, will try client-side OCR as fallback
        }
      }

      // Fall back to client-side OCR if server OCR unavailable or failed
      // NOTE: Client-side ML Kit OCR is currently configured for Latin script.
      // Japanese cards scan best with server-side OCR (EasyOCR with ja+en).
      if (scanResult == null || scanResult.cards.isEmpty) {
        try {
          // Use client-side ML Kit OCR as fallback
          final ocrResult = await _ocrService.processImage(image.path);

          if (ocrResult.textLines.isNotEmpty) {
            // Send full OCR text to server for parsing
            final fullText = ocrResult.textLines.join('\n');

            scanResult = await _apiService.identifyCard(
              fullText,
              _selectedGame,
              imageAnalysis: ocrResult.imageAnalysis,
            );
            usedServerOCR = false;

            // Show warning if using client OCR and scanning Pokemon.
            if (mounted &&
                !_shownJapaneseFallbackWarning &&
                _selectedGame == 'pokemon' &&
                _serverOCRAvailable != true) {
              _shownJapaneseFallbackWarning = true;
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text(
                    'Using fallback OCR - Japanese cards may not scan correctly',
                  ),
                  duration: Duration(seconds: 3),
                ),
              );
            }
          }
        } on OcrException {
          // Client OCR also failed
          if (scanResult == null) {
            rethrow;
          }
        }
      }

      // Clean up temp image file
      await File(image.path).delete();

      if (!mounted) return;

      if (scanResult == null || scanResult.cards.isEmpty) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('No cards found')));
        return;
      }

      // Navigate to results - best match should be first
      final result = scanResult;
      final detectedCardName = result.metadata.cardName ?? '';

      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (context) => ScanResultScreen(
            cards: result.cards,
            searchQuery: detectedCardName.isNotEmpty
                ? detectedCardName
                : (usedServerOCR
                      ? 'Scanned Card (Server OCR)'
                      : 'Scanned Card'),
            game: _selectedGame,
            scanMetadata: result.metadata,
            setIcon: result.setIcon,
            scannedImageBytes: imageBytesAsList,
            grouped: result.grouped, // For MTG 2-phase selection
          ),
        ),
      );
    } catch (e) {
      if (mounted) {
        // Show user-friendly error message
        String message = 'An error occurred';
        final errorStr = e.toString();
        if (errorStr.contains('timed out')) {
          message = 'Request timed out. Check your connection.';
        } else if (errorStr.contains('SocketException') ||
            errorStr.contains('Connection')) {
          message = 'Cannot connect to server. Check your network.';
        } else if (errorStr.contains('No text detected')) {
          message =
              'No text detected in image. Try again with better lighting.';
        } else {
          message = 'Error: ${e.toString().replaceAll('Exception: ', '')}';
        }
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(message)));
      }
    } finally {
      if (mounted) {
        setState(() => _isProcessing = false);
      }
    }
  }

  @override
  void dispose() {
    _qualityCheckTimer?.cancel();
    _controller?.dispose();
    _ocrService.dispose();
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
          // Camera preview with card guide overlay
          Expanded(
            child: Center(
              child: _isInitialized && _controller != null
                  ? ClipRRect(
                      borderRadius: BorderRadius.circular(12),
                      child: Stack(
                        fit: StackFit.expand,
                        children: [
                          CameraPreview(_controller!),
                          const CardGuideOverlay(),
                        ],
                      ),
                    )
                  : const CircularProgressIndicator(),
            ),
          ),
          // Quality feedback
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: 16.0,
              vertical: 8.0,
            ),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              decoration: BoxDecoration(
                color: _isQualityAcceptable
                    ? const Color.fromRGBO(
                        76,
                        175,
                        80,
                        0.8,
                      ) // Colors.green with 0.8 alpha
                    : const Color.fromRGBO(
                        255,
                        152,
                        0,
                        0.8,
                      ), // Colors.orange with 0.8 alpha
                borderRadius: BorderRadius.circular(20),
              ),
              child: Text(
                _qualityFeedback,
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          ),
          // Capture button
          Padding(
            padding: const EdgeInsets.all(24.0),
            child: Center(
              child: FloatingActionButton.large(
                onPressed: _isProcessing ? null : _captureAndProcess,
                child: _isProcessing
                    ? const CircularProgressIndicator(color: Colors.white)
                    : const Icon(Icons.camera_alt),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Overlay widget that draws a card guide frame on the camera preview
class CardGuideOverlay extends StatelessWidget {
  final double aspectRatio;

  const CardGuideOverlay({super.key, this.aspectRatio = cardAspectRatio});

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        // Calculate card frame size - use 80% of the smaller dimension
        final maxWidth = constraints.maxWidth * 0.8;
        final maxHeight = constraints.maxHeight * 0.75;

        // Calculate actual card dimensions based on aspect ratio
        double cardWidth;
        double cardHeight;

        if (maxWidth / aspectRatio <= maxHeight) {
          cardWidth = maxWidth;
          cardHeight = maxWidth / aspectRatio;
        } else {
          cardHeight = maxHeight;
          cardWidth = maxHeight * aspectRatio;
        }

        return CustomPaint(
          size: Size(constraints.maxWidth, constraints.maxHeight),
          painter: CardGuidePainter(
            cardWidth: cardWidth,
            cardHeight: cardHeight,
          ),
        );
      },
    );
  }
}

/// Custom painter that draws the card guide frame
class CardGuidePainter extends CustomPainter {
  final double cardWidth;
  final double cardHeight;

  CardGuidePainter({required this.cardWidth, required this.cardHeight});

  @override
  void paint(Canvas canvas, Size size) {
    final centerX = size.width / 2;
    final centerY = size.height / 2;

    final cardRect = Rect.fromCenter(
      center: Offset(centerX, centerY),
      width: cardWidth,
      height: cardHeight,
    );

    // Draw semi-transparent overlay outside the card area
    final overlayPath = Path()
      ..addRect(Rect.fromLTWH(0, 0, size.width, size.height))
      ..addRRect(RRect.fromRectAndRadius(cardRect, const Radius.circular(12)))
      ..fillType = PathFillType.evenOdd;

    final overlayPaint = Paint()
      ..color = const Color.fromRGBO(0, 0, 0, 0.5)
      ..style = PaintingStyle.fill;

    canvas.drawPath(overlayPath, overlayPaint);

    // Draw card frame border
    final borderPaint = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.stroke
      ..strokeWidth = 3.0;

    canvas.drawRRect(
      RRect.fromRectAndRadius(cardRect, const Radius.circular(12)),
      borderPaint,
    );

    // Draw corner markers for visual alignment
    const cornerLength = 30.0;
    const cornerOffset = 8.0;
    final cornerPaint = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.stroke
      ..strokeWidth = 4.0
      ..strokeCap = StrokeCap.round;

    // Top-left corner
    canvas.drawLine(
      Offset(cardRect.left - cornerOffset, cardRect.top + cornerLength),
      Offset(cardRect.left - cornerOffset, cardRect.top - cornerOffset),
      cornerPaint,
    );
    canvas.drawLine(
      Offset(cardRect.left - cornerOffset, cardRect.top - cornerOffset),
      Offset(cardRect.left + cornerLength, cardRect.top - cornerOffset),
      cornerPaint,
    );

    // Top-right corner
    canvas.drawLine(
      Offset(cardRect.right - cornerLength, cardRect.top - cornerOffset),
      Offset(cardRect.right + cornerOffset, cardRect.top - cornerOffset),
      cornerPaint,
    );
    canvas.drawLine(
      Offset(cardRect.right + cornerOffset, cardRect.top - cornerOffset),
      Offset(cardRect.right + cornerOffset, cardRect.top + cornerLength),
      cornerPaint,
    );

    // Bottom-left corner
    canvas.drawLine(
      Offset(cardRect.left - cornerOffset, cardRect.bottom - cornerLength),
      Offset(cardRect.left - cornerOffset, cardRect.bottom + cornerOffset),
      cornerPaint,
    );
    canvas.drawLine(
      Offset(cardRect.left - cornerOffset, cardRect.bottom + cornerOffset),
      Offset(cardRect.left + cornerLength, cardRect.bottom + cornerOffset),
      cornerPaint,
    );

    // Bottom-right corner
    canvas.drawLine(
      Offset(cardRect.right - cornerLength, cardRect.bottom + cornerOffset),
      Offset(cardRect.right + cornerOffset, cardRect.bottom + cornerOffset),
      cornerPaint,
    );
    canvas.drawLine(
      Offset(cardRect.right + cornerOffset, cardRect.bottom + cornerOffset),
      Offset(cardRect.right + cornerOffset, cardRect.bottom - cornerLength),
      cornerPaint,
    );
  }

  @override
  bool shouldRepaint(CardGuidePainter oldDelegate) {
    return oldDelegate.cardWidth != cardWidth ||
        oldDelegate.cardHeight != cardHeight;
  }
}
