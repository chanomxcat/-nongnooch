import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../core/api.dart';
import '../core/format.dart';
import '../core/tickets.dart';
import '../main.dart';
import '../theme.dart';
import 'login.dart';

class TicketsScreen extends StatelessWidget {
  const TicketsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(l.ticketsTitle)),
      body: LoginGate(builder: (_) => const _TicketList()),
    );
  }
}

class _TicketList extends StatefulWidget {
  const _TicketList();
  @override
  State<_TicketList> createState() => _TicketListState();
}

class _TicketListState extends State<_TicketList> {
  Future<TicketsView>? _f;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _f ??= _fetch();
  }

  Future<TicketsView> _fetch() {
    final d = Deps.of(context);
    return loadTickets(d.api, d.tickets);
  }

  Future<void> _reload() async {
    final f = _fetch();
    setState(() => _f = f);
    await f.catchError((_) => TicketsView(const []));
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    return FutureBuilder<TicketsView>(
      future: _f,
      builder: (c, s) {
        if (s.hasError) {
          final e = s.error;
          return Center(
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              Text(e is ApiError && e.status == 0 ? l.errorNetwork : l.errorGeneric),
              TextButton(onPressed: _reload, child: Text(l.retry)),
            ]),
          );
        }
        if (!s.hasData) return const Center(child: CircularProgressIndicator());
        final v = s.data!;
        return RefreshIndicator(
          onRefresh: _reload,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              if (v.stale && v.syncedAt != null)
                Container(
                  margin: const EdgeInsets.only(bottom: 12),
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(color: NnColors.gold100, borderRadius: BorderRadius.circular(NnRadius.medium)),
                  child: Row(children: [
                    const Icon(Icons.cloud_off, size: 18, color: NnColors.green900),
                    const SizedBox(width: 8),
                    Expanded(child: Text(l.ticketsLastUpdated(formatBangkok(v.syncedAt!)))),
                    IconButton(onPressed: _reload, icon: const Icon(Icons.refresh), tooltip: l.retry),
                  ]),
                ),
              if (v.tickets.isEmpty)
                Padding(padding: const EdgeInsets.all(32), child: Center(child: Text(l.ticketsEmpty))),
              for (final t in v.tickets) ...[TicketCard(t), const SizedBox(height: 16)],
            ],
          ),
        );
      },
    );
  }
}

class TicketCard extends StatelessWidget {
  const TicketCard(this.t, {super.key});
  final Ticket t;

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final status = t.used ? l.ticketUsed : l.ticketUnused;
    return Container(
      decoration: BoxDecoration(
        color: NnColors.white,
        borderRadius: BorderRadius.circular(NnRadius.mobileCard),
        boxShadow: nnShadow3,
      ),
      padding: const EdgeInsets.all(16),
      child: Column(children: [
        Row(children: [
          Expanded(
            child: Text(t.kind == 'annual' ? l.ticketAnnual : l.ticketDay,
                style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: NnColors.green900)),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
            decoration: BoxDecoration(
              color: t.used ? NnColors.grey100 : NnColors.green900,
              borderRadius: BorderRadius.circular(999),
            ),
            child: Text(status,
                style: TextStyle(
                    fontSize: 12, fontWeight: FontWeight.w700, color: t.used ? NnColors.textSubtle : NnColors.white)),
          ),
        ]),
        const SizedBox(height: 12),
        // The QR is the exact NN1. string from the server, unmodified. Used tickets are dimmed.
        Semantics(
          label: 'QR ${maskLast4(t.id)}',
          image: true,
          child: Opacity(
            opacity: t.used ? 0.35 : 1,
            child: Container(
              color: NnColors.white,
              padding: const EdgeInsets.all(8),
              child: QrImageView(data: t.qr, size: 260, backgroundColor: NnColors.white),
            ),
          ),
        ),
        const SizedBox(height: 12),
        _row(l.ticketHolder, '${t.holderName}  ${maskLast4(t.idLast4)}'),
        if (t.visitDate != null) _row(l.ticketVisitDate, t.visitDate!),
        _row('#', maskLast4(t.id)),
      ]),
    );
  }

  Widget _row(String k, String v) => Padding(
        padding: const EdgeInsets.only(top: 4),
        child: Row(children: [
          Text(k, style: const TextStyle(color: NnColors.textSubtle)),
          const SizedBox(width: 12),
          Expanded(
            child: Text(v, textAlign: TextAlign.end, overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontWeight: FontWeight.w600)),
          ),
        ]),
      );
}
