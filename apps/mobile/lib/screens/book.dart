import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/api.dart';
import '../core/checkout.dart';
import '../core/format.dart';
import '../main.dart';
import '../theme.dart';
import 'login.dart';

// ponytail: guest = OTP-verified, foreign price only; Plan 15 Task 5 full checkout (companions, Thai price, waiting room).
class BookScreen extends StatefulWidget {
  const BookScreen({super.key, this.paidOrder, required this.onPaid});

  /// Order id from the KBank return URL (`?paid=<id>`): poll it instead of showing the picker.
  final String? paidOrder;
  final VoidCallback onPaid;

  @override
  State<BookScreen> createState() => _BookScreenState();
}

enum _Step { pick, holders, polling, failed }

class _BookScreenState extends State<BookScreen> {
  Future<List<Map>>? _f;
  String? _lang;
  _Step _step = _Step.pick;
  final _qty = <String, int>{};
  List<(String, Holder)> _items = [];
  Checkout? _checkout;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    if (widget.paidOrder != null) {
      _step = _Step.polling;
      WidgetsBinding.instance.addPostFrameCallback((_) => _poll(widget.paidOrder!));
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final lang = Localizations.localeOf(context).languageCode;
    if (lang != _lang) {
      _lang = lang;
      _f = _fetch();
    }
  }

  Future<List<Map>> _fetch() => Deps.of(context)
      .api
      .get('/api/commerce/products?lang=$_lang')
      .then((j) => List<Map>.from((j as Map)['items'] as List)
          .where((p) => (p['kind'] ?? 'day') == 'day' && p['foreignSatang'] != null)
          .toList());

  void _load() => setState(() => _f = _fetch());

  Future<void> _poll(String id) async {
    final ok = await waitPaid(Deps.of(context).api, id);
    if (!mounted) return;
    if (ok) {
      setState(() => _step = _Step.pick);
      widget.onPaid();
    } else {
      setState(() => _step = _Step.failed);
    }
  }

  void _toHolders(List<Map> products) {
    _items = [
      for (final p in products)
        for (var i = 0; i < (_qty[p['productId']] ?? 0); i++) ('${p['productId']}', Holder()),
    ];
    _checkout = null;
    setState(() {
      _step = _Step.holders;
      _error = null;
    });
  }

  Future<void> _pay() async {
    final l = L.of(context);
    if (!_items.every((e) => e.$2.valid)) return setState(() => _error = l.bookHolderInvalid);
    setState(() {
      _busy = true;
      _error = null;
    });
    _checkout ??= Checkout(Deps.of(context).api);
    try {
      final url = await _checkout!.pay(buildOrderRequest(_items), Uri.base.origin);
      await launchUrl(Uri.parse(url), webOnlyWindowName: '_self');
    } on ApiError catch (e) {
      // Network errors keep the same idempotency keys for the retry; a rejection starts over.
      if (e.status != 0) _checkout = null;
      setState(() => _error = e.status == 0 ? l.errorNetwork : '${l.errorGeneric} (${e.code})');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final session = Deps.of(context).session;
    return Scaffold(
      appBar: AppBar(
        title: Text(l.bookTitle),
        leading: _step == _Step.holders
            ? IconButton(icon: const Icon(Icons.arrow_back), onPressed: () => setState(() => _step = _Step.pick))
            : null,
      ),
      body: switch (_step) {
        _Step.polling => Center(
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              const CircularProgressIndicator(),
              const SizedBox(height: 16),
              Text(l.bookPaying),
            ]),
          ),
        _Step.failed => Center(
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              Text(l.bookPayFailed, style: const TextStyle(color: NnColors.error)),
              const SizedBox(height: 12),
              FilledButton(onPressed: () => setState(() => _step = _Step.pick), child: Text(l.retry)),
            ]),
          ),
        _Step.holders => ListenableBuilder(
            listenable: session,
            builder: (c, _) => session.loggedIn ? _holders(l) : const LoginForm(),
          ),
        _Step.pick => _picker(l),
      },
    );
  }

  Widget _holders(L l) => ListView(
        padding: const EdgeInsets.all(16),
        children: [
          for (final (i, (_, h)) in _items.indexed)
            Card(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Column(children: [
                  Align(alignment: Alignment.centerLeft, child: Text(l.bookHolderN(i + 1))),
                  TextField(
                    decoration: InputDecoration(labelText: l.bookHolderName),
                    onChanged: (v) => h.name = v,
                  ),
                  DropdownButtonFormField<String>(
                    value: h.nationality,
                    decoration: InputDecoration(labelText: l.bookHolderNationality),
                    items: [for (final c in nationalities) DropdownMenuItem(value: c, child: Text(c))],
                    onChanged: (v) => h.nationality = v ?? h.nationality,
                  ),
                  TextField(
                    decoration: InputDecoration(labelText: l.bookHolderIdLast4),
                    keyboardType: TextInputType.number,
                    maxLength: 4,
                    onChanged: (v) => h.idLast4 = v.trim(),
                  ),
                ]),
              ),
            ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(_error!, style: const TextStyle(color: NnColors.error)),
            ),
          const SizedBox(height: 8),
          FilledButton(style: goldButton(), onPressed: _busy ? null : _pay, child: Text(l.bookPay)),
        ],
      );

  Widget _picker(L l) => FutureBuilder<List<Map>>(
        future: _f,
        builder: (c, s) {
          if (s.hasError) {
            return Center(
              child: Column(mainAxisSize: MainAxisSize.min, children: [
                Text(l.errorNetwork),
                TextButton(onPressed: _load, child: Text(l.retry)),
              ]),
            );
          }
          if (!s.hasData) return const Center(child: CircularProgressIndicator());
          final total = _qty.values.fold(0, (a, b) => a + b);
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              for (final p in s.data!) ...[
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Row(children: [
                      Expanded(
                        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Text('${p['name']}',
                              style: const TextStyle(
                                  fontSize: 18, fontWeight: FontWeight.w600, color: NnColors.green900)),
                          const SizedBox(height: 8),
                          _Price(l.bookForeignPrice, p['foreignSatang'] as int? ?? 0),
                        ]),
                      ),
                      IconButton(
                        tooltip: '-',
                        icon: const Icon(Icons.remove_circle_outline),
                        onPressed: (_qty['${p['productId']}'] ?? 0) == 0
                            ? null
                            : () => setState(() => _qty['${p['productId']}'] = _qty['${p['productId']}']! - 1),
                      ),
                      Text('${_qty['${p['productId']}'] ?? 0}', style: const TextStyle(fontSize: 18)),
                      IconButton(
                        tooltip: '+',
                        icon: const Icon(Icons.add_circle_outline),
                        onPressed: total >= 20
                            ? null
                            : () => setState(() => _qty['${p['productId']}'] = (_qty['${p['productId']}'] ?? 0) + 1),
                      ),
                    ]),
                  ),
                ),
                const SizedBox(height: 12),
              ],
              Text(l.bookThaiWeb, style: const TextStyle(fontSize: 12, color: NnColors.textSubtle)),
              const SizedBox(height: 16),
              FilledButton(
                style: goldButton(),
                onPressed: total == 0 ? null : () => _toHolders(s.data!),
                child: Text(l.bookBuy),
              ),
            ],
          );
        },
      );
}

class _Price extends StatelessWidget {
  const _Price(this.label, this.satang);
  final String label;
  final int satang;
  @override
  Widget build(BuildContext context) => Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text(label, style: const TextStyle(fontSize: 12, color: NnColors.textSubtle)),
        Text(formatSatang(satang), style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700)),
      ]);
}
