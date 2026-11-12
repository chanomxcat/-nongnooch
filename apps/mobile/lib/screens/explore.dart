import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:url_launcher/url_launcher.dart';

import '../theme.dart';
import 'common.dart';

class ExploreScreen extends StatelessWidget {
  const ExploreScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    // ponytail: emergency numbers are national hotlines bundled in the app; the CMS contract has no type for them yet.
    final emergency = [
      (l.emergencyMedical, '1669'),
      (l.emergencyTouristPolice, '1155'),
      (l.emergencyPolice, '191'),
    ];
    return Scaffold(
      appBar: AppBar(title: Text(l.tabExplore)),
      body: ListView(
        padding: const EdgeInsets.only(bottom: 24),
        children: [
          SectionTitle(l.exploreEmergency),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Card(
              child: Column(children: [
                for (final (label, phone) in emergency)
                  ListTile(
                    leading: const Icon(Icons.local_phone, color: NnColors.error),
                    title: Text(label),
                    trailing: Text(phone,
                        style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: NnColors.green900)),
                    onTap: () => launchUrl(Uri.parse('tel:$phone')),
                  ),
              ]),
            ),
          ),
          SectionTitle(l.exploreFaq),
          ContentBuilder(
            type: 'faq',
            builder: (c, r) => Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Column(children: [
                Card(
                  clipBehavior: Clip.antiAlias,
                  child: Column(children: [
                    for (final f in r.items)
                      ExpansionTile(
                        title: Text(f.t('question'), style: const TextStyle(fontWeight: FontWeight.w600)),
                        childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                        expandedAlignment: Alignment.centerLeft,
                        children: [Text(f.t('answer'))],
                      ),
                  ]),
                ),
                if (!r.fromCms) const OfflineBadge(),
              ]),
            ),
          ),
        ],
      ),
    );
  }
}
