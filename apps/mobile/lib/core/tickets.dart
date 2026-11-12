import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';

class Ticket {
  Ticket(this.raw);
  final Map<String, dynamic> raw;

  String get id => raw['ticketId'] as String? ?? '';
  String get kind => raw['kind'] as String? ?? '';
  String? get visitDate => raw['visitDate'] as String?;
  String get holderName => raw['holderName'] as String? ?? '';
  String get idLast4 => raw['idLast4'] as String? ?? '';
  String get status => raw['status'] as String? ?? '';
  bool get used => status == 'used';

  /// The exact `NN1.<payload>.<sig>` string from the server. Never modified.
  String get qr => raw['qr'] as String? ?? '';
}

class TicketsView {
  TicketsView(this.tickets, {this.stale = false, this.syncedAt});
  final List<Ticket> tickets;
  final bool stale;
  final DateTime? syncedAt;
}

/// Last known ticket list in local storage (localStorage on web), raw JSON so `qr` stays byte-identical.
class TicketCache {
  TicketCache(this.prefs);
  final SharedPreferences prefs;
  static const _k = 'tickets_v1';

  Future<void> save(List<Map<String, dynamic>> items, DateTime at) =>
      prefs.setString(_k, jsonEncode({'at': at.toUtc().toIso8601String(), 'items': items}));

  TicketsView? read() {
    final s = prefs.getString(_k);
    if (s == null) return null;
    try {
      final j = jsonDecode(s) as Map;
      final items = (j['items'] as List).map((e) => Ticket(Map<String, dynamic>.from(e as Map))).toList();
      return TicketsView(items, stale: true, syncedAt: DateTime.parse(j['at'] as String));
    } catch (_) {
      return null; // corrupted cache: behave as empty, never crash
    }
  }
}

/// Network first; on failure show the cached list marked stale.
Future<TicketsView> loadTickets(Api api, TicketCache cache, {DateTime Function()? now}) async {
  try {
    final j = await api.get('/api/ticket/tickets/me', auth: true);
    final items = ((j as Map)['items'] as List? ?? const []).map((e) => Map<String, dynamic>.from(e as Map)).toList();
    final at = (now ?? DateTime.now)();
    await cache.save(items, at);
    return TicketsView(items.map(Ticket.new).toList(), syncedAt: at);
  } on ApiError catch (e) {
    if (e.status == 401) rethrow;
    final cached = cache.read();
    if (cached != null) return cached;
    rethrow;
  }
}
