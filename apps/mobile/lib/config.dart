/// Build-time config: pass with --dart-define (see deploy/Dockerfile.app).
class Config {
  /// Gateway base, the app calls `$apiBase/api/<svc>/...`.
  static const apiBase = String.fromEnvironment('API_BASE', defaultValue: 'http://localhost:8000');

  /// Public website, used for relative CMS image paths and the checkout link.
  static const webBase = String.fromEnvironment('WEB_BASE', defaultValue: 'http://localhost:3000');

  /// Mock console for OTP SMS in demo mode.
  static const mockextBase = String.fromEnvironment('MOCKEXT_BASE', defaultValue: 'http://localhost:8090');

  /// Demo mode: OTP hint, Turnstile `mock-ok`, semantics on for e2e.
  static const demo = bool.fromEnvironment('DEMO', defaultValue: true);

  static const langs = ['th', 'en', 'zh', 'ja', 'ko', 'de', 'fr'];
  static const langNames = {
    'th': 'ไทย', 'en': 'English', 'zh': '中文', 'ja': '日本語', 'ko': '한국어', 'de': 'Deutsch', 'fr': 'Français',
  };
}
