import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// App-wide state persisted in local storage: device id, language and tokens.
class Session extends ChangeNotifier {
  Session(this.prefs) {
    deviceId = prefs.getString(_kDevice) ?? _uuidV4();
    prefs.setString(_kDevice, deviceId);
  }

  final SharedPreferences prefs;
  late final String deviceId;

  static const _kDevice = 'device_id';
  static const _kLang = 'lang';
  static const _kAccess = 'access';
  static const _kRefresh = 'refresh';

  String get lang => prefs.getString(_kLang) ?? 'th';
  String? get access => prefs.getString(_kAccess);
  String? get refresh => prefs.getString(_kRefresh);
  bool get loggedIn => access != null;

  Future<void> setLang(String l) async {
    await prefs.setString(_kLang, l);
    notifyListeners();
  }

  Future<void> setTokens(String access, String refresh) async {
    await prefs.setString(_kAccess, access);
    await prefs.setString(_kRefresh, refresh);
    notifyListeners();
  }

  Future<void> logout() async {
    await prefs.remove(_kAccess);
    await prefs.remove(_kRefresh);
    await prefs.remove('tickets_v1');
    notifyListeners();
  }

  static String _uuidV4() {
    final r = Random.secure();
    final b = List<int>.generate(16, (_) => r.nextInt(256));
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    final h = b.map((x) => x.toRadixString(16).padLeft(2, '0')).join();
    return '${h.substring(0, 8)}-${h.substring(8, 12)}-${h.substring(12, 16)}-${h.substring(16, 20)}-${h.substring(20)}';
  }
}
