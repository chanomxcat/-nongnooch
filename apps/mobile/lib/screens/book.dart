import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:url_launcher/url_launcher.dart';

import '../config.dart';
import '../core/format.dart';
import '../main.dart';
import '../theme.dart';

class BookScreen extends StatefulWidget {
  const BookScreen({super.key});
  @override
  State<BookScreen> createState() => _BookScreenState();
}

class _BookScreenState extends State<BookScreen> {
  Future<List<Map>>? _f;
  String? _lang;

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
      .then((j) => List<Map>.from((j as Map)['items'] as List));

  void _load() => setState(() => _f = _fetch());

  void _buy() {
    // ponytail: in-app checkout is Plan 15 Task 5; the demo hands off to the website checkout in a new tab.
    launchUrl(Uri.parse('${Config.webBase}/$_lang/checkout'), webOnlyWindowName: '_blank');
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(l.bookTitle)),
      body: FutureBuilder<List<Map>>(
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
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              for (final p in s.data!) ...[
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text('${p['name']}',
                          style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w600, color: NnColors.green900)),
                      const SizedBox(height: 8),
                      Row(children: [
                        Expanded(child: _Price(l.bookThaiPrice, p['thaiSatang'] as int? ?? 0)),
                        Expanded(child: _Price(l.bookForeignPrice, p['foreignSatang'] as int? ?? 0)),
                      ]),
                    ]),
                  ),
                ),
                const SizedBox(height: 12),
              ],
              Text(l.bookThaiNote, style: const TextStyle(fontSize: 12, color: NnColors.textSubtle)),
              const SizedBox(height: 16),
              FilledButton.icon(
                style: goldButton(),
                onPressed: _buy,
                icon: const Icon(Icons.open_in_new, size: 18),
                label: Text(l.bookBuy),
              ),
            ],
          );
        },
      ),
    );
  }
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
