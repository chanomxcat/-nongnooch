import 'dart:convert';

import 'package:http/http.dart' as http;

import '../config.dart';
import 'session.dart';

class ApiError implements Exception {
  ApiError(this.status, this.code);
  final int status; // 0 = network
  final String code;
  @override
  String toString() => 'ApiError($status, $code)';
}

/// Thin gateway client: X-Device-Id + Accept-Language on every call, Bearer on authed ones,
/// one refresh-and-retry on 401.
class Api {
  Api(this.session, {http.Client? client, String? base})
      : _client = client ?? http.Client(),
        base = base ?? Config.apiBase;

  final Session session;
  final http.Client _client;
  final String base;

  Future<dynamic> get(String path, {bool auth = false}) => send('GET', path, auth: auth);
  Future<dynamic> post(String path, Object body, {bool auth = false, Map<String, String>? headers}) =>
      send('POST', path, body: body, auth: auth, headers: headers);

  Future<dynamic> send(String method, String path,
      {Object? body, bool auth = false, Map<String, String>? headers}) async {
    var r = await _do(method, path, body, auth, headers);
    if (auth && r.statusCode == 401 && await _refresh()) {
      r = await _do(method, path, body, auth, headers);
    }
    if (auth && r.statusCode == 401) await session.logout();
    if (r.statusCode < 200 || r.statusCode >= 300) {
      var code = 'http_${r.statusCode}';
      try {
        code = (jsonDecode(utf8.decode(r.bodyBytes)) as Map)['error'] as String? ?? code;
      } catch (_) {}
      throw ApiError(r.statusCode, code);
    }
    if (r.bodyBytes.isEmpty) return null;
    return jsonDecode(utf8.decode(r.bodyBytes));
  }

  Future<http.Response> _do(String method, String path, Object? body, bool auth,
      [Map<String, String>? extra]) async {
    final h = <String, String>{
      ...?extra,
      'X-Device-Id': session.deviceId,
      'Accept-Language': session.lang,
      if (body != null) 'Content-Type': 'application/json',
      if (auth && session.access != null) 'Authorization': 'Bearer ${session.access}',
    };
    final req = http.Request(method, Uri.parse('$base$path'))..headers.addAll(h);
    if (body != null) req.body = jsonEncode(body);
    try {
      return await http.Response.fromStream(await _client.send(req).timeout(const Duration(seconds: 10)));
    } catch (e) {
      throw ApiError(0, 'network');
    }
  }

  Future<bool> _refresh() async {
    final rt = session.refresh;
    if (rt == null) return false;
    try {
      final r = await _do('POST', '/api/identity/auth/refresh', {'refreshToken': rt}, false);
      if (r.statusCode != 200) return false;
      final j = jsonDecode(utf8.decode(r.bodyBytes)) as Map;
      await session.setTokens(j['access'] as String, j['refresh'] as String);
      return true;
    } on ApiError {
      return false;
    }
  }

  // ---- Auth ----
  Future<String> requestOtp(String phone) async {
    final j = await post('/api/identity/auth/otp', {
      'phone': phone,
      'lang': session.lang,
      'turnstileToken': 'mock-ok', // ponytail: demo Turnstile is mockext; real widget comes with Plan 15 Task 3.
    });
    return j['requestId'] as String;
  }

  Future<void> verifyOtp(String requestId, String otp) async {
    final j = await post('/api/identity/auth/verify', {'requestId': requestId, 'otp': otp});
    await session.setTokens(j['access'] as String, j['refresh'] as String);
  }
}
