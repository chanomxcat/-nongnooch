import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:nongnooch_app/core/api.dart';
import 'package:nongnooch_app/core/content.dart';
import 'package:nongnooch_app/core/session.dart';
import 'package:nongnooch_app/core/tickets.dart';
import 'package:shared_preferences/shared_preferences.dart';

const qr = 'NN1.eyJ0IjoiMDFhMWNiOGYiLCJrIjoiZGF5In0.wWmIxOFbIHPlTjiXVhvYjKrq-_7387FKY';

Future<(Session, SharedPreferences)> session([Map<String, Object> init = const {}]) async {
  SharedPreferences.setMockInitialValues(init);
  final p = await SharedPreferences.getInstance();
  return (Session(p), p);
}

http.Response json(Object body, [int status = 200]) =>
    http.Response.bytes(utf8.encode(jsonEncode(body)), status, headers: {'content-type': 'application/json'});

void main() {
  group('CMS content with fallback', () {
    test('API failure -> bundled content', () async {
      final (s, _) = await session();
      final api = Api(s, base: 'http://x', client: MockClient((r) async => json({'error': 'bad_gateway'}, 502)));
      final r = await ContentRepo(api).load('hero_slide', 'th');
      expect(r.fromCms, isFalse);
      expect(r.items, isNotEmpty);
      expect(r.items.first.t('title'), 'สวนสวยระดับโลก');
    });

    test('empty API list -> bundled content; unknown lang falls back to en', () async {
      final (s, _) = await session();
      final api = Api(s, base: 'http://x', client: MockClient((r) async => json({'items': []})));
      final r = await ContentRepo(api).load('faq', 'de');
      expect(r.fromCms, isFalse);
      expect(r.items.first.t('question'), 'What are the opening hours?');
    });

    test('CMS items win when present, request carries lang + headers', () async {
      final (s, _) = await session();
      late http.Request seen;
      final api = Api(s, base: 'http://x', client: MockClient((r) async {
        seen = r;
        return json({
          'items': [
            {'id': '1', 'type': 'show', 'attrs': {'order': 1, 'times': ['09:00']}, 'texts': {'name': 'จาก CMS'}}
          ]
        });
      }));
      final r = await ContentRepo(api).load('show', 'th');
      expect(r.fromCms, isTrue);
      expect(r.items.single.t('name'), 'จาก CMS');
      expect(r.items.single.times, ['09:00']);
      expect(seen.url.toString(), 'http://x/api/content/public/items?type=show&lang=th');
      expect(seen.headers['X-Device-Id'], s.deviceId);
      expect(seen.headers['Accept-Language'], 'th');
    });
  });

  group('ticket cache', () {
    final item = {'ticketId': 'abc-1234', 'kind': 'day', 'visitDate': '2026-11-12', 'holderName': 'Somchai', 'idLast4': '0708', 'status': 'active', 'qr': qr};

    test('network ok -> saves cache, qr byte-identical', () async {
      final (s, p) = await session({'access': 'a', 'refresh': 'r'});
      final api = Api(s, base: 'http://x', client: MockClient((r) async {
        expect(r.headers['Authorization'], 'Bearer a');
        return json({'items': [item]});
      }));
      final v = await loadTickets(api, TicketCache(p), now: () => DateTime.utc(2026, 11, 12, 1));
      expect(v.stale, isFalse);
      expect(v.tickets.single.qr, qr);
      final cached = TicketCache(p).read()!;
      expect(cached.tickets.single.qr, qr);
      expect(cached.syncedAt, DateTime.utc(2026, 11, 12, 1));
    });

    test('network down -> cached list marked stale with time', () async {
      final (s, p) = await session({'access': 'a', 'refresh': 'r'});
      await TicketCache(p).save([item], DateTime.utc(2026, 11, 12, 1));
      final api = Api(s, base: 'http://x', client: MockClient((r) async => throw Exception('offline')));
      final v = await loadTickets(api, TicketCache(p));
      expect(v.stale, isTrue);
      expect(v.syncedAt, DateTime.utc(2026, 11, 12, 1));
      expect(v.tickets.single.qr, qr);
    });

    test('network down and no cache -> error', () async {
      final (s, p) = await session({'access': 'a', 'refresh': 'r'});
      final api = Api(s, base: 'http://x', client: MockClient((r) async => throw Exception('offline')));
      expect(() => loadTickets(api, TicketCache(p)), throwsA(isA<ApiError>()));
    });

    test('corrupted cache reads as empty, does not throw', () async {
      final (_, p) = await session({'tickets_v1': '{not json'});
      expect(TicketCache(p).read(), isNull);
    });

    test('401 -> refresh once and retry', () async {
      final (s, p) = await session({'access': 'old', 'refresh': 'r1'});
      final calls = <String>[];
      final api = Api(s, base: 'http://x', client: MockClient((r) async {
        calls.add('${r.method} ${r.url.path} ${r.headers['Authorization'] ?? ''}');
        if (r.url.path.endsWith('/refresh')) return json({'access': 'new', 'refresh': 'r2'});
        if (r.headers['Authorization'] == 'Bearer old') return json({'error': 'invalid_token'}, 401);
        return json({'items': [item]});
      }));
      final v = await loadTickets(api, TicketCache(p));
      expect(v.tickets, hasLength(1));
      expect(calls, [
        'GET /api/ticket/tickets/me Bearer old',
        'POST /api/identity/auth/refresh ',
        'GET /api/ticket/tickets/me Bearer new',
      ]);
      expect(s.refresh, 'r2');
    });
  });
}
