import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:nongnooch_app/core/api.dart';
import 'package:nongnooch_app/core/content.dart';
import 'package:nongnooch_app/core/session.dart';
import 'package:nongnooch_app/core/tickets.dart';
import 'package:nongnooch_app/main.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('offline cold start: cached tickets with QR, status and last-updated label', (t) async {
    const qr = 'NN1.eyJ0IjoiYSJ9.c2ln';
    SharedPreferences.setMockInitialValues({'access': 'a', 'refresh': 'r', 'lang': 'th'});
    final p = await SharedPreferences.getInstance();
    await TicketCache(p).save([
      {'ticketId': 'x-9999', 'kind': 'day', 'visitDate': '2026-11-12', 'holderName': 'Somchai', 'idLast4': '0708', 'status': 'used', 'qr': qr},
      {'ticketId': 'y-8888', 'kind': 'annual', 'holderName': 'Somchai', 'idLast4': '0708', 'status': 'active', 'qr': qr},
    ], DateTime.utc(2026, 11, 12, 1, 30));
    t.view.physicalSize = const Size(400, 3000);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    final s = Session(p);
    final api = Api(s, base: 'http://x', client: MockClient((r) async => throw Exception('offline')));
    await t.pumpWidget(NongnoochApp(deps: Deps(s, api, ContentRepo(api), TicketCache(p))));
    await t.pumpAndSettle();

    // Home shows bundled hero content when the CMS is unreachable.
    expect(find.text('สวนสวยระดับโลก'), findsOneWidget);

    await t.tap(find.descendant(of: find.byType(NavigationBar), matching: find.text('บัตรของฉัน')));
    await t.pumpAndSettle();
    expect(find.text('อัปเดตล่าสุดเมื่อ 12/11/2026 08:30'), findsOneWidget);
    expect(find.text('ใช้แล้ว'), findsOneWidget);
    expect(find.text('ยังไม่ใช้'), findsOneWidget);
    expect(find.text('Somchai  ****0708'), findsWidgets);
    final qrs = t.widgetList<QrImageView>(find.byType(QrImageView, skipOffstage: false));
    expect(qrs.length, 2);
  });
}
