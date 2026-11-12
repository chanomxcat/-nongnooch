import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';

import '../config.dart';
import '../core/format.dart';
import '../main.dart';
import '../theme.dart';
import 'login.dart';

class MoreScreen extends StatelessWidget {
  const MoreScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final s = Deps.of(context).session;
    return Scaffold(
      appBar: AppBar(title: Text(l.tabMore)),
      body: ListView(
        padding: const EdgeInsets.only(bottom: 24),
        children: [
          ListenableBuilder(
            listenable: s,
            builder: (c, _) => s.loggedIn ? const _Points() : const SizedBox.shrink(),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
            child: Text(l.moreLanguage, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: NnColors.green900)),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Wrap(spacing: 8, runSpacing: 8, children: [
              for (final code in Config.langs)
                ChoiceChip(
                  label: Text(Config.langNames[code]!),
                  selected: s.lang == code,
                  selectedColor: NnColors.gold500,
                  labelStyle: TextStyle(color: s.lang == code ? NnColors.green900 : NnColors.textDefault),
                  onSelected: (_) => s.setLang(code),
                ),
            ]),
          ),
          const SizedBox(height: 24),
          ListenableBuilder(
            listenable: s,
            builder: (c, _) => s.loggedIn
                ? Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    child: OutlinedButton.icon(
                      onPressed: s.logout,
                      icon: const Icon(Icons.logout),
                      label: Text(l.moreLogout),
                    ),
                  )
                : const SizedBox(height: 520, child: LoginForm()),
          ),
        ],
      ),
    );
  }
}

class _Points extends StatefulWidget {
  const _Points();
  @override
  State<_Points> createState() => _PointsState();
}

class _PointsState extends State<_Points> {
  Future<(Map, Map)>? _f;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final api = Deps.of(context).api;
    _f ??= Future.wait([
      api.get('/api/member/members/me', auth: true),
      api.get('/api/member/members/me/points', auth: true),
    ]).then((r) => (r[0] as Map, r[1] as Map));
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return FutureBuilder<(Map, Map)>(
      future: _f,
      builder: (c, s) {
        if (s.hasError) return Padding(padding: const EdgeInsets.all(16), child: Text(l.errorNetwork));
        if (!s.hasData) return const Padding(padding: EdgeInsets.all(24), child: Center(child: CircularProgressIndicator()));
        final (me, pts) = s.data!;
        final items = (pts['items'] as List? ?? const []).cast<Map>();
        return Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(color: NnColors.green900, borderRadius: BorderRadius.circular(NnRadius.mobileCard)),
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text('${l.moreMember} · ${me['firstName'] ?? ''} ${me['lastName'] ?? ''}',
                    style: const TextStyle(color: NnColors.gold300)),
                const SizedBox(height: 8),
                Text(l.morePoints, style: const TextStyle(color: NnColors.white)),
                Text('${pts['available'] ?? 0}',
                    style: const TextStyle(color: NnColors.white, fontSize: 40, fontWeight: FontWeight.w700)),
                const SizedBox(height: 8),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                  decoration: BoxDecoration(color: NnColors.gold500, borderRadius: BorderRadius.circular(999)),
                  child: Text(l.moreRedeemSoon,
                      style: const TextStyle(color: NnColors.green900, fontWeight: FontWeight.w700)),
                ),
              ]),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(0, 16, 0, 8),
              child: Text(l.morePointsHistory,
                  style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: NnColors.green900)),
            ),
            Card(
              child: Column(children: [
                for (final p in items)
                  ListTile(
                    title: Text(switch ('${p['reason']}') {
                      'welcome' => l.pointsWelcome,
                      'purchase' || 'order' || 'order_paid' => l.pointsPurchase,
                      'invite' || 'referral' => l.pointsInvite,
                      final r => r,
                    }),
                    subtitle: Text(formatBangkok(DateTime.parse(p['createdAt'] as String))),
                    trailing: Text(
                      '${(p['deltaPoints'] as int) > 0 ? '+' : ''}${p['deltaPoints']}',
                      style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: NnColors.success),
                    ),
                  ),
              ]),
            ),
          ]),
        );
      },
    );
  }
}
