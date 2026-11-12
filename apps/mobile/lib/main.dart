import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'config.dart';
import 'core/api.dart';
import 'core/content.dart';
import 'core/session.dart';
import 'core/tickets.dart';
import 'screens/book.dart';
import 'screens/explore.dart';
import 'screens/home.dart';
import 'screens/more.dart';
import 'screens/tickets.dart';
import 'theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // Demo builds expose the semantics tree (aria labels) so Playwright can select by text.
  if (Config.demo) SemanticsBinding.instance.ensureSemantics();
  final prefs = await SharedPreferences.getInstance();
  final session = Session(prefs);
  final api = Api(session);
  runApp(NongnoochApp(deps: Deps(session, api, ContentRepo(api), TicketCache(prefs))));
}

/// Plain dependency bag, reachable from any screen via `Deps.of(context)`.
class Deps {
  Deps(this.session, this.api, this.content, this.tickets);
  final Session session;
  final Api api;
  final ContentRepo content;
  final TicketCache tickets;

  static Deps of(BuildContext c) => c.dependOnInheritedWidgetOfExactType<_DepsScope>()!.deps;
}

class _DepsScope extends InheritedWidget {
  const _DepsScope({required this.deps, required super.child});
  final Deps deps;
  @override
  bool updateShouldNotify(_DepsScope old) => false;
}

class NongnoochApp extends StatelessWidget {
  const NongnoochApp({super.key, required this.deps});
  final Deps deps;

  @override
  Widget build(BuildContext context) {
    return _DepsScope(
      deps: deps,
      child: ListenableBuilder(
        listenable: deps.session,
        builder: (context, _) => MaterialApp(
          debugShowCheckedModeBanner: false,
          onGenerateTitle: (c) => L.of(c).appTitle,
          theme: buildTheme(),
          locale: Locale(deps.session.lang),
          supportedLocales: L.supportedLocales,
          localizationsDelegates: L.localizationsDelegates,
          builder: (context, child) => PhoneFrame(child: child!),
          home: const Shell(),
        ),
      ),
    );
  }
}

/// On wide screens, show the app in a centered ~420px phone-width frame.
class PhoneFrame extends StatelessWidget {
  const PhoneFrame({super.key, required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final mq = MediaQuery.of(context);
    if (mq.size.width <= 600) return child;
    const w = 420.0;
    final h = mq.size.height > 940 ? 900.0 : mq.size.height - 32;
    return ColoredBox(
      color: NnColors.green950,
      child: Center(
        child: Container(
          width: w,
          height: h,
          clipBehavior: Clip.antiAlias,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(32),
            border: Border.all(color: NnColors.gold500, width: 3),
          ),
          child: MediaQuery(data: mq.copyWith(size: Size(w, h)), child: child),
        ),
      ),
    );
  }
}

class Shell extends StatefulWidget {
  const Shell({super.key});
  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  int _tab = 0;

  void go(int i) => setState(() => _tab = i);

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final pages = [
      HomeScreen(onTab: go),
      const ExploreScreen(),
      const BookScreen(),
      const TicketsScreen(),
      const MoreScreen(),
    ];
    return Scaffold(
      body: IndexedStack(index: _tab, children: pages),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: go,
        backgroundColor: NnColors.white,
        indicatorColor: NnColors.grey100,
        destinations: [
          NavigationDestination(icon: const Icon(Icons.home_outlined), selectedIcon: const Icon(Icons.home), label: l.tabHome),
          NavigationDestination(icon: const Icon(Icons.map_outlined), selectedIcon: const Icon(Icons.map), label: l.tabExplore),
          NavigationDestination(icon: const Icon(Icons.calendar_today_outlined), selectedIcon: const Icon(Icons.calendar_today), label: l.tabBook),
          NavigationDestination(icon: const Icon(Icons.confirmation_number_outlined), selectedIcon: const Icon(Icons.confirmation_number), label: l.tabTickets),
          NavigationDestination(icon: const Icon(Icons.menu), label: l.tabMore),
        ],
      ),
    );
  }
}
