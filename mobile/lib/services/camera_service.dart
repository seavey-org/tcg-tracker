import 'package:camera/camera.dart';

/// Abstraction layer for camera operations to enable testing
class CameraService {
  /// Get list of available cameras on the device
  Future<List<CameraDescription>> getAvailableCameras() async {
    return availableCameras();
  }

  /// Create a camera controller for the given camera
  CameraController createController(
    CameraDescription camera, {
    ResolutionPreset resolutionPreset = ResolutionPreset.high,
    bool enableAudio = false,
  }) {
    return CameraController(
      camera,
      resolutionPreset,
      enableAudio: enableAudio,
    );
  }
}
