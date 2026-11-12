import 'package:flutter_test/flutter_test.dart';
import 'package:nongnooch_app/core/format.dart';

void main() {
  group('normalizePhone', () {
    test('Thai local number -> E.164', () {
      expect(normalizePhone('0811111111'), '+66811111111');
      expect(normalizePhone('081-111-1111'), '+66811111111');
      expect(normalizePhone(' 081 111 1111 '), '+66811111111');
    });
    test('already international', () {
      expect(normalizePhone('+66811111111'), '+66811111111');
      expect(normalizePhone('66811111111'), '+66811111111');
      expect(normalizePhone('+8613800138000'), '+8613800138000');
      expect(normalizePhone('008613800138000'), '+8613800138000');
    });
    test('invalid', () {
      expect(normalizePhone(''), isNull);
      expect(normalizePhone('12345'), isNull);
      expect(normalizePhone('08111'), isNull);
      expect(normalizePhone('+0123456789'), isNull);
    });
  });

  group('formatSatang', () {
    test('whole baht with separators', () {
      expect(formatSatang(30000), '฿300');
      expect(formatSatang(150000), '฿1,500');
      expect(formatSatang(123456700), '฿1,234,567');
      expect(formatSatang(0), '฿0');
    });
    test('satang fractions', () {
      expect(formatSatang(12345), '฿123.45');
      expect(formatSatang(5), '฿0.05');
      expect(formatSatang(-150050), '-฿1,500.50');
    });
  });

  test('maskLast4 never shows more than 4 chars', () {
    expect(maskLast4('1234567890123'), '****0123');
    expect(maskLast4('0708'), '****0708');
  });

  test('formatBangkok is UTC+7', () {
    expect(formatBangkok(DateTime.utc(2026, 11, 11, 23, 5)), '12/11/2026 06:05');
  });
}
