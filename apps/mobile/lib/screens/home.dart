import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';

import '../core/content.dart';
import '../main.dart';
import '../theme.dart';
import 'common.dart';

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key, required this.onTab});
  final void Function(int) onTab;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return ListView(
      padding: EdgeInsets.zero,
      children: [
        ContentBuilder(type: 'hero_slide', builder: (c, r) => _Hero(r)),
        _MyTicketCard(onTab: onTab),
        _QuickLinks(onTab: onTab),
        SectionTitle(l.homeTodayShows),
        ContentBuilder(
          type: 'show',
          builder: (c, r) => Column(children: [
            for (final s in r.items) _ShowCard(s),
            if (!r.fromCms) const OfflineBadge(),
          ]),
        ),
        const SizedBox(height: 24),
      ],
    );
  }
}

class _Hero extends StatefulWidget {
  const _Hero(this.r);
  final ContentResult r;
  @override
  State<_Hero> createState() => _HeroState();
}

class _HeroState extends State<_Hero> {
  int _i = 0;

  @override
  Widget build(BuildContext context) {
    final items = widget.r.items;
    return SizedBox(
      height: 330,
      child: Stack(children: [
        PageView.builder(
          itemCount: items.length,
          onPageChanged: (i) => setState(() => _i = i),
          itemBuilder: (c, i) {
            final s = items[i];
            return Stack(fit: StackFit.expand, children: [
              NnImage(s.imageUrl),
              const DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [Color(0x0000271C), Color(0xE600271C)],
                  ),
                ),
              ),
              Positioned(
                left: 20,
                right: 20,
                bottom: 40,
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(s.t('title'),
                      style: const TextStyle(color: NnColors.white, fontSize: 30, fontWeight: FontWeight.w700)),
                  Text(s.t('subtitle'), style: const TextStyle(color: NnColors.gold300, fontSize: 16)),
                ]),
              ),
            ]);
          },
        ),
        Positioned(
          left: 20,
          bottom: 20,
          child: Row(children: [
            for (var i = 0; i < items.length; i++)
              Container(
                width: i == _i ? 28 : 16,
                height: 3,
                margin: const EdgeInsets.only(right: 6),
                decoration: BoxDecoration(
                  color: i == _i ? NnColors.gold500 : NnColors.white.withOpacity(0.6),
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
          ]),
        ),
      ]),
    );
  }
}

class _MyTicketCard extends StatelessWidget {
  const _MyTicketCard({required this.onTab});
  final void Function(int) onTab;

  @override
  Widget build(BuildContext context) {
    final d = Deps.of(context);
    final l = L.of(context);
    return ListenableBuilder(
      listenable: d.session,
      builder: (c, _) {
        final cached = d.session.loggedIn ? d.tickets.read() : null;
        final t = cached?.tickets.where((t) => !t.used).firstOrNull ?? cached?.tickets.firstOrNull;
        if (t == null) return const SizedBox.shrink();
        return Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
          child: Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: NnColors.white,
              borderRadius: BorderRadius.circular(NnRadius.mobileCard),
              boxShadow: nnShadow3,
            ),
            child: Row(children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(color: NnColors.green900, borderRadius: BorderRadius.circular(NnRadius.medium)),
                child: const Icon(Icons.confirmation_number_outlined, color: NnColors.white),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(l.homeMyTicket,
                      style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: NnColors.green900)),
                  Text(t.kind == 'annual' ? l.ticketAnnual : l.ticketDay,
                      style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w500)),
                  Text('${t.visitDate ?? ''} · ${cached!.tickets.length}',
                      style: const TextStyle(fontSize: 12, color: NnColors.textSubtle)),
                ]),
              ),
              FilledButton.icon(
                style: goldButton().copyWith(minimumSize: const WidgetStatePropertyAll(Size(0, 40))),
                onPressed: () => onTab(3),
                icon: const Icon(Icons.qr_code, size: 18),
                label: Text(l.homeShowQr),
              ),
            ]),
          ),
        );
      },
    );
  }
}

class _QuickLinks extends StatelessWidget {
  const _QuickLinks({required this.onTab});
  final void Function(int) onTab;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final links = [
      (Icons.calendar_today, l.homeQuickBook, 2),
      (Icons.confirmation_number_outlined, l.homeQuickTickets, 3),
      (Icons.help_outline, l.homeQuickExplore, 1),
      (Icons.stars_outlined, l.homeQuickPoints, 4),
    ];
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 16),
        decoration: BoxDecoration(
          color: NnColors.white,
          borderRadius: BorderRadius.circular(NnRadius.mobileCard),
          boxShadow: nnShadow1,
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceEvenly,
          children: [
            for (final (icon, label, tab) in links)
              InkWell(
                onTap: () => onTab(tab),
                borderRadius: BorderRadius.circular(NnRadius.medium),
                child: Column(children: [
                  Container(
                    width: 52,
                    height: 52,
                    decoration: BoxDecoration(color: NnColors.green900, borderRadius: BorderRadius.circular(NnRadius.large)),
                    child: Icon(icon, color: NnColors.gold300),
                  ),
                  const SizedBox(height: 6),
                  Text(label, style: const TextStyle(fontSize: 13)),
                ]),
              ),
          ],
        ),
      ),
    );
  }
}

class _ShowCard extends StatelessWidget {
  const _ShowCard(this.s);
  final ContentItem s;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
      child: Card(
        clipBehavior: Clip.antiAlias,
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 104, height: 112, child: NnImage(s.imageUrl)),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(s.t('name'), style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: NnColors.green900)),
                Text(s.t('venue'), style: const TextStyle(fontSize: 12, color: NnColors.textSubtle)),
                const SizedBox(height: 8),
                Wrap(spacing: 6, runSpacing: 6, children: [
                  for (final t in s.times)
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 2),
                      decoration: BoxDecoration(color: NnColors.gold100, borderRadius: BorderRadius.circular(999)),
                      child: Text(t, style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: NnColors.green900)),
                    ),
                ]),
              ]),
            ),
          ),
        ]),
      ),
    );
  }
}
