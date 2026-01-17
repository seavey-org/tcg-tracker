import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Common test setup utilities

/// Sets up SharedPreferences with mock initial values
Future<void> setupMockSharedPreferences([Map<String, Object> values = const {}]) async {
  SharedPreferences.setMockInitialValues(values);
}

/// Sets up SharedPreferences with a server URL
Future<void> setupMockServerUrl(String url) async {
  await setupMockSharedPreferences({'server_url': url});
}

/// Sets up SharedPreferences with default (empty) values
Future<void> setupEmptySharedPreferences() async {
  await setupMockSharedPreferences({});
}

/// Creates a MaterialApp wrapper for widget testing
Widget createTestableWidget(Widget child) {
  return MaterialApp(
    home: child,
  );
}

/// Creates a MaterialApp wrapper with navigation support
Widget createTestableWidgetWithNavigation(
  Widget child, {
  Map<String, WidgetBuilder>? routes,
}) {
  return MaterialApp(
    home: child,
    routes: routes ?? {},
  );
}

/// Creates a MaterialApp wrapper with theme support
Widget createTestableWidgetWithTheme(
  Widget child, {
  ThemeData? theme,
}) {
  return MaterialApp(
    theme: theme ?? ThemeData.light(useMaterial3: true),
    home: child,
  );
}

/// Creates a Scaffold-wrapped testable widget
Widget createScaffoldTestableWidget(Widget child) {
  return MaterialApp(
    home: Scaffold(body: child),
  );
}
