import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';

import '../core/content.dart';
import '../main.dart';
import '../theme.dart';

/// CMS image (bundled asset, website path or absolute URL) with a plain placeholder.
class NnImage extends StatelessWidget {
  const NnImage(this.url, {super.key, this.fit = BoxFit.cover});
  final String? url;
  final BoxFit fit;

  @override
  Widget build(BuildContext context) {
    const ph = ColoredBox(color: NnColors.green900);
    final u = url;
    if (u == null || u.isEmpty) return ph;
    final r = resolveImage(u);
    if (r.startsWith('asset:')) return Image.asset(r.substring(6), fit: fit, errorBuilder: (_, __, ___) => ph);
    return Image.network(r, fit: fit, errorBuilder: (_, __, ___) => ph);
  }
}

/// Loads one CMS type for the current language and reloads on language change.
class ContentBuilder extends StatefulWidget {
  const ContentBuilder({super.key, required this.type, required this.builder});
  final String type;
  final Widget Function(BuildContext, ContentResult) builder;
  @override
  State<ContentBuilder> createState() => _ContentBuilderState();
}

class _ContentBuilderState extends State<ContentBuilder> {
  Future<ContentResult>? _f;
  String? _lang;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final d = Deps.of(context);
    final lang = Localizations.localeOf(context).languageCode;
    if (lang != _lang) {
      _lang = lang;
      _f = d.content.load(widget.type, lang);
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<ContentResult>(
      future: _f,
      builder: (c, s) {
        if (!s.hasData) return const Padding(padding: EdgeInsets.all(24), child: Center(child: CircularProgressIndicator()));
        return widget.builder(c, s.data!);
      },
    );
  }
}

class OfflineBadge extends StatelessWidget {
  const OfflineBadge({super.key});
  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        child: Row(children: [
          const Icon(Icons.cloud_off, size: 14, color: NnColors.textDisabled),
          const SizedBox(width: 4),
          Text(L.of(context).contentOffline, style: const TextStyle(fontSize: 11, color: NnColors.textDisabled)),
        ]),
      );
}

class SectionTitle extends StatelessWidget {
  const SectionTitle(this.text, {super.key});
  final String text;
  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
        child: Text(text, style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: NnColors.green900)),
      );
}
