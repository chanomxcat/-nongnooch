import 'api.dart';
import 'session.dart';

// ponytail: guest = OTP-verified, foreign price only; Plan 15 Task 5 full checkout (companions, Thai price, waiting room).

class Holder {
  Holder({this.name = '', this.nationality = 'CN', this.idLast4 = ''});
  String name, nationality, idLast4;
  bool get valid => name.trim().isNotEmpty && RegExp(r'^\d{4}$').hasMatch(idLast4);
}

const nationalities = ['CN', 'US', 'GB', 'JP', 'KR', 'DE', 'FR', 'IN', 'RU', 'AU', 'MY', 'SG', 'TH'];

/// Today in Asia/Bangkok (UTC+7, no DST) as YYYY-MM-DD.
String bangkokToday([DateTime? now]) =>
    (now ?? DateTime.now()).toUtc().add(const Duration(hours: 7)).toIso8601String().substring(0, 10);

/// POST /api/commerce/orders body: one foreign-price item per holder, in product order.
Map<String, Object> buildOrderRequest(List<(String productId, Holder holder)> items, {DateTime? now}) => {
      'visitDate': bangkokToday(now),
      'items': [
        for (final (pid, h) in items)
          {
            'productId': pid,
            'audience': 'foreign',
            'holder': {'name': h.name.trim(), 'nationality': h.nationality, 'idLast4': h.idLast4},
          }
      ],
    };

/// Creates the order then the KBank payment; returns (orderId, paymentUrl).
/// Keys are kept per checkout attempt, so a retry of the same attempt reuses them.
class Checkout {
  Checkout(this.api);
  final Api api;
  final orderKey = Session.uuidV4();
  final payKey = Session.uuidV4();
  String? orderId;

  Future<String> pay(Map<String, Object> order, String returnBase) async {
    orderId ??= ((await api.post('/api/commerce/orders', order, auth: true, headers: {'Idempotency-Key': orderKey}))
        as Map)['orderId'] as String;
    final j = await api.post('/api/commerce/orders/$orderId/payments',
        {'method': 'card', 'returnUrl': '$returnBase/?paid=$orderId'},
        auth: true, headers: {'Idempotency-Key': payKey}) as Map;
    return (j['paymentUrl'] ?? j['redirectUrl'] ?? j['url']) as String;
  }
}

/// Polls GET /orders/{id} every 2s up to 60s. true = paid/confirmed, false = failed/expired/timeout.
Future<bool> waitPaid(Api api, String id,
    {Duration every = const Duration(seconds: 2), Duration max = const Duration(seconds: 60)}) async {
  final end = DateTime.now().add(max);
  while (DateTime.now().isBefore(end)) {
    try {
      final s = ((await api.get('/api/commerce/orders/$id', auth: true)) as Map)['status'];
      if (s == 'paid' || s == 'confirmed') return true;
      if (s == 'expired' || s == 'cancelled' || s == 'failed') return false;
    } on ApiError catch (e) {
      if (e.status == 404 || e.status == 401) return false;
    }
    await Future.delayed(every);
  }
  return false;
}
