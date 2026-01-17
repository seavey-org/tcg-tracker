import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// Extension on WidgetTester to provide convenience methods for pumping widgets
extension PumpApp on WidgetTester {
  /// Pumps a widget wrapped in MaterialApp
  Future<void> pumpApp(Widget widget) async {
    await pumpWidget(
      MaterialApp(
        home: widget,
      ),
    );
  }

  /// Pumps a widget wrapped in MaterialApp with routes
  Future<void> pumpAppWithRoutes(
    Widget widget, {
    Map<String, WidgetBuilder>? routes,
    String? initialRoute,
  }) async {
    await pumpWidget(
      MaterialApp(
        home: widget,
        routes: routes ?? {},
        initialRoute: initialRoute,
      ),
    );
  }

  /// Pumps a widget wrapped in MaterialApp and settles all animations
  Future<void> pumpAppAndSettle(Widget widget) async {
    await pumpApp(widget);
    await pumpAndSettle();
  }

  /// Pumps a widget wrapped in Scaffold inside MaterialApp
  Future<void> pumpScaffoldApp(Widget widget) async {
    await pumpWidget(
      MaterialApp(
        home: Scaffold(body: widget),
      ),
    );
  }

  /// Pumps a widget with a custom theme
  Future<void> pumpThemedApp(
    Widget widget, {
    ThemeData? theme,
    ThemeData? darkTheme,
    ThemeMode themeMode = ThemeMode.light,
  }) async {
    await pumpWidget(
      MaterialApp(
        theme: theme ?? ThemeData.light(useMaterial3: true),
        darkTheme: darkTheme,
        themeMode: themeMode,
        home: widget,
      ),
    );
  }
}
