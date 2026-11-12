/// Normalizes a phone number to E.164. Thai local numbers (0XXXXXXXXX) become +66XXXXXXXXX.
/// Returns null when the input cannot be a valid phone number.
String? normalizePhone(String input) {
  final s = input.trim().replaceAll(RegExp(r'[\s\-().]'), '');
  String out;
  if (s.startsWith('+')) {
    out = s;
  } else if (s.startsWith('00')) {
    out = '+${s.substring(2)}';
  } else if (s.startsWith('0') && s.length == 10) {
    out = '+66${s.substring(1)}';
  } else if (s.startsWith('66') && s.length == 11) {
    out = '+$s';
  } else {
    return null;
  }
  return RegExp(r'^\+[1-9]\d{7,14}$').hasMatch(out) ? out : null;
}

/// Formats an integer amount in satang as THB, e.g. 150000 -> "฿1,500", 12345 -> "฿123.45".
String formatSatang(int satang) {
  final neg = satang < 0;
  final abs = satang.abs();
  final baht = abs ~/ 100;
  final st = abs % 100;
  final digits = baht.toString();
  final buf = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buf.write(',');
    buf.write(digits[i]);
  }
  final dec = st == 0 ? '' : '.${st.toString().padLeft(2, '0')}';
  return '${neg ? '-' : ''}฿$buf$dec';
}

/// "2026-11-12T08:05:00Z" -> "12/11/2026 15:05" in Asia/Bangkok (UTC+7, no DST).
String formatBangkok(DateTime t) {
  final b = t.toUtc().add(const Duration(hours: 7));
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(b.day)}/${two(b.month)}/${b.year} ${two(b.hour)}:${two(b.minute)}';
}

/// Last 4 characters only, masked: "****0708". Never shows a full ID.
String maskLast4(String v) {
  final s = v.length > 4 ? v.substring(v.length - 4) : v;
  return '****$s';
}
