import 'package:flutter_test/flutter_test.dart';
import 'package:nongnooch_app/core/checkout.dart';

void main() {
  test('guest order request: foreign price, ad-hoc holder, Bangkok date', () {
    // 2026-11-12 18:30 UTC is already 2026-11-13 in Bangkok.
    final r = buildOrderRequest([
      ('p1', Holder(name: ' Li Wei ', nationality: 'CN', idLast4: '1234')),
    ], now: DateTime.utc(2026, 11, 12, 18, 30));
    expect(r, {
      'visitDate': '2026-11-13',
      'items': [
        {
          'productId': 'p1',
          'audience': 'foreign',
          'holder': {'name': 'Li Wei', 'nationality': 'CN', 'idLast4': '1234'},
        }
      ],
    });
    expect(Holder(name: 'A', idLast4: '12a4').valid, isFalse);
    expect(nationalities.every((c) => c.length == 2 && c != 'XX'), isTrue);
  });
}
