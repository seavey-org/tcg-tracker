import 'package:flutter/material.dart';

final ColorScheme _lightColorScheme =
    ColorScheme.fromSeed(
      seedColor: const Color(0xFFDC2626),
      brightness: Brightness.light,
    ).copyWith(
      primary: const Color(0xFFDC2626),
      onPrimary: Colors.white,
      primaryContainer: const Color(0xFFFEE2E2),
      onPrimaryContainer: const Color(0xFF7F1D1D),
      secondary: const Color(0xFF4B5563),
      onSecondary: Colors.white,
      secondaryContainer: const Color(0xFFE5E7EB),
      onSecondaryContainer: const Color(0xFF1F2937),
      tertiary: const Color(0xFF22C55E),
      onTertiary: Colors.white,
      error: const Color(0xFFEF4444),
      onError: Colors.white,
      errorContainer: const Color(0xFFFEE2E2),
      onErrorContainer: const Color(0xFF7F1D1D),
      surface: const Color(0xFFFFFFFF),
      onSurface: const Color(0xFF1F2937),
      onSurfaceVariant: const Color(0xFF4B5563),
      outline: const Color(0xFFD1D5DB),
      outlineVariant: const Color(0xFFE5E7EB),
      inversePrimary: const Color(0xFFEF4444),
    );

final ColorScheme _darkColorScheme =
    ColorScheme.fromSeed(
      seedColor: const Color(0xFFEF4444),
      brightness: Brightness.dark,
    ).copyWith(
      primary: const Color(0xFFEF4444),
      onPrimary: Colors.white,
      primaryContainer: const Color(0xFF7F1D1D),
      onPrimaryContainer: const Color(0xFFFEE2E2),
      secondary: const Color(0xFFD1D5DB),
      onSecondary: const Color(0xFF0A0A0A),
      secondaryContainer: const Color(0xFF262626),
      onSecondaryContainer: const Color(0xFFD1D5DB),
      tertiary: const Color(0xFF4ADE80),
      onTertiary: const Color(0xFF0A0A0A),
      error: const Color(0xFFF87171),
      onError: const Color(0xFF0A0A0A),
      errorContainer: const Color(0xFF7F1D1D),
      onErrorContainer: const Color(0xFFFECACA),
      surface: const Color(0xFF171717),
      onSurface: const Color(0xFFFFFFFF),
      onSurfaceVariant: const Color(0xFFD1D5DB),
      outline: const Color(0xFF525252),
      outlineVariant: const Color(0xFF404040),
      inversePrimary: const Color(0xFFDC2626),
    );

final ThemeData lightTheme = ThemeData(
  colorScheme: _lightColorScheme,
  scaffoldBackgroundColor: const Color(0xFFF3F4F6),
  canvasColor: const Color(0xFFF3F4F6),
  cardColor: const Color(0xFFFFFFFF),
  appBarTheme: const AppBarTheme(
    backgroundColor: Color(0xFFFFFFFF),
    foregroundColor: Color(0xFF1F2937),
  ),
  useMaterial3: true,
);

final ThemeData darkTheme = ThemeData(
  colorScheme: _darkColorScheme,
  scaffoldBackgroundColor: const Color(0xFF0A0A0A),
  canvasColor: const Color(0xFF0A0A0A),
  cardColor: const Color(0xFF171717),
  appBarTheme: const AppBarTheme(
    backgroundColor: Color(0xFF171717),
    foregroundColor: Color(0xFFFFFFFF),
  ),
  useMaterial3: true,
);
