import 'package:flutter/material.dart';

/// Tokens from docs/design/DESIGN.md section 2. Use these names; no raw colors in screens.
class NnColors {
  static const green900 = Color(0xFF074536); // brand/primary
  static const green950 = Color(0xFF00271C);
  static const primarySubtle = Color(0xFFF5F7F6);
  static const gold500 = Color(0xFFD9A52E); // brand/secondary
  static const gold600 = Color(0xFFB58612); // secondary-hover
  static const gold300 = Color(0xFFE9CA8E);
  static const gold100 = Color(0xFFF6EAD2);
  static const secondarySubtle = Color(0xFFFBF6EC);
  static const emerald400 = Color(0xFF7CC489);
  static const textDefault = Color(0xFF2A2E37);
  static const textSubtle = Color(0xFF5A606D);
  static const textDisabled = Color(0xFF898E9A);
  static const white = Color(0xFFFFFFFF);
  static const surfaceSubtle = Color(0xFFF6F7F8);
  static const border = Color(0xFFC3C6CC);
  static const grey100 = Color(0xFFDFE1E4);
  static const success = Color(0xFF16A34A);
  static const error = Color(0xFFDC2626);
}

class NnRadius {
  static const medium = 12.0;
  static const large = 16.0;
  static const mobileCard = 20.0;
  static const xlarge = 24.0;
}

const nnShadow1 = [BoxShadow(color: Color(0x14071C16), offset: Offset(0, 1), blurRadius: 2)];
const nnShadow3 = [BoxShadow(color: Color(0x1F071C16), offset: Offset(0, 4), blurRadius: 8)];

/// Browser fonts (HTML renderer): Noto Sans Thai if installed, else the OS Thai/CJK font.
const _fallbackFonts = ['Noto Sans Thai', 'Noto Sans', 'Leelawadee UI', 'Thonburi', 'Tahoma', 'sans-serif'];

ThemeData buildTheme() {
  final base = ThemeData(
    useMaterial3: true,
    colorScheme: ColorScheme.fromSeed(
      seedColor: NnColors.green900,
      primary: NnColors.green900,
      onPrimary: NnColors.white,
      secondary: NnColors.gold500,
      onSecondary: NnColors.green900,
      surface: NnColors.white,
      onSurface: NnColors.textDefault,
      error: NnColors.error,
    ),
    scaffoldBackgroundColor: NnColors.surfaceSubtle,
    fontFamily: 'Noto Sans Thai',
    fontFamilyFallback: _fallbackFonts,
  );
  return base.copyWith(
    appBarTheme: const AppBarTheme(
      backgroundColor: NnColors.green900,
      foregroundColor: NnColors.white,
      centerTitle: false,
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: NnColors.green900,
        foregroundColor: NnColors.white,
        minimumSize: const Size(0, 48),
        shape: const StadiumBorder(),
        textStyle: const TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
      ),
    ),
    cardTheme: CardTheme(
      color: NnColors.white,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(NnRadius.large)),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: NnColors.white,
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(NnRadius.medium)),
    ),
  );
}

/// Gold CTA: text is green/900, never white (DESIGN.md).
ButtonStyle goldButton() => FilledButton.styleFrom(
      backgroundColor: NnColors.gold500,
      foregroundColor: NnColors.green900,
      minimumSize: const Size(0, 48),
      shape: const StadiumBorder(),
      textStyle: const TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
    );
