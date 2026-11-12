import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:url_launcher/url_launcher.dart';

import '../config.dart';
import '../core/api.dart';
import '../core/format.dart';
import '../main.dart';
import '../theme.dart';

/// Shows [child] when logged in, otherwise the OTP login form. Rebuilds on login / logout.
class LoginGate extends StatelessWidget {
  const LoginGate({super.key, required this.builder});
  final WidgetBuilder builder;

  @override
  Widget build(BuildContext context) {
    final s = Deps.of(context).session;
    return ListenableBuilder(
      listenable: s,
      builder: (c, _) => s.loggedIn ? builder(c) : const LoginForm(),
    );
  }
}

class LoginForm extends StatefulWidget {
  const LoginForm({super.key});
  @override
  State<LoginForm> createState() => _LoginFormState();
}

class _LoginFormState extends State<LoginForm> {
  final _phone = TextEditingController();
  final _otp = TextEditingController();
  String? _requestId;
  String? _e164;
  String? _error;
  bool _busy = false;

  Future<void> _send() async {
    final l = L.of(context);
    final p = normalizePhone(_phone.text);
    if (p == null) return setState(() => _error = l.loginInvalidPhone);
    setState(() { _busy = true; _error = null; });
    try {
      final id = await Deps.of(context).api.requestOtp(p);
      setState(() { _requestId = id; _e164 = p; });
    } on ApiError catch (e) {
      setState(() => _error = e.status == 0 ? l.errorNetwork : '${l.errorGeneric} (${e.code})');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    final l = L.of(context);
    setState(() { _busy = true; _error = null; });
    try {
      await Deps.of(context).api.verifyOtp(_requestId!, _otp.text.trim());
    } on ApiError catch (e) {
      setState(() => _error = e.status == 0 ? l.errorNetwork : l.loginWrongOtp);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = L.of(context);
    final sent = _requestId != null;
    return SafeArea(
      child: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          const SizedBox(height: 24),
          const Icon(Icons.local_florist, size: 56, color: NnColors.green900),
          const SizedBox(height: 12),
          Text(l.loginTitle, textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w600, color: NnColors.green900)),
          const SizedBox(height: 24),
          if (!sent) ...[
            TextField(
              controller: _phone,
              keyboardType: TextInputType.phone,
              decoration: InputDecoration(labelText: l.loginPhoneLabel, hintText: l.loginPhoneHint),
              onSubmitted: (_) => _send(),
            ),
            const SizedBox(height: 16),
            FilledButton(onPressed: _busy ? null : _send, child: Text(l.loginSendOtp)),
          ] else ...[
            Text(l.loginOtpSentTo(_e164!), textAlign: TextAlign.center),
            const SizedBox(height: 16),
            TextField(
              controller: _otp,
              keyboardType: TextInputType.number,
              maxLength: 6,
              decoration: InputDecoration(labelText: l.loginOtpLabel),
              onSubmitted: (_) => _verify(),
            ),
            const SizedBox(height: 8),
            FilledButton(onPressed: _busy ? null : _verify, child: Text(l.loginVerify)),
            TextButton(
              onPressed: () => setState(() { _requestId = null; _otp.clear(); }),
              child: Text(l.loginChangePhone),
            ),
          ],
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: NnColors.error)),
            ),
          if (Config.demo) ...[
            const SizedBox(height: 24),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: NnColors.secondarySubtle,
                borderRadius: BorderRadius.circular(NnRadius.medium),
                border: Border.all(color: NnColors.gold300),
              ),
              child: InkWell(
                onTap: () => launchUrl(Uri.parse(Config.mockextBase), webOnlyWindowName: '_blank'),
                child: Text(l.loginDemoHint, style: const TextStyle(color: NnColors.textSubtle)),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
