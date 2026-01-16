import 'package:flutter/material.dart';
import 'screens/camera_screen.dart';
import 'screens/settings_screen.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const TCGTrackerApp());
}

class TCGTrackerApp extends StatelessWidget {
  const TCGTrackerApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'TCG Tracker',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
        useMaterial3: true,
      ),
      home: const CameraScreen(),
      routes: {
        '/settings': (context) => const SettingsScreen(),
      },
    );
  }
}
