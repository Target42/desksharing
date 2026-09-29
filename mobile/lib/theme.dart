import 'package:flutter/material.dart';

const ink = Color(0xFF14221F);
const accent = Color(0xFF0F7A65);
const accentDark = Color(0xFF0C6554);
const panel = Color(0xFFF7F3EA);
const canvas = Color(0xFFF3EFE6);
const forest = Color(0xFF17302B);
const danger = Color(0xFF9B2C2C);
const ok = Color(0xFF1F7A4D);

ThemeData buildTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: accent,
    brightness: Brightness.light,
  );
  return ThemeData(
    colorScheme: scheme.copyWith(
      primary: accent,
      onPrimary: Colors.white,
      error: danger,
    ),
    scaffoldBackgroundColor: canvas,
    useMaterial3: true,
    appBarTheme: const AppBarTheme(
      backgroundColor: forest,
      foregroundColor: panel,
      elevation: 0,
      centerTitle: false,
    ),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: panel,
      indicatorColor: accent.withValues(alpha: 0.16),
      labelTextStyle: WidgetStateProperty.resolveWith((states) {
        final selected = states.contains(WidgetState.selected);
        return TextStyle(
          fontSize: 12,
          fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
          color: selected ? accentDark : ink.withValues(alpha: 0.7),
        );
      }),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: accent,
        foregroundColor: Colors.white,
        minimumSize: const Size.fromHeight(48),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: Colors.white,
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
    ),
    cardTheme: CardThemeData(
      color: panel,
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: ink.withValues(alpha: 0.08)),
      ),
    ),
  );
}
