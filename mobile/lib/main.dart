import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'screens/login_screen.dart';
import 'screens/shell_screen.dart';
import 'session.dart';
import 'theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final prefs = await SharedPreferences.getInstance();
  runApp(DesksharingApp(session: Session(prefs)));
}

class DesksharingApp extends StatefulWidget {
  const DesksharingApp({super.key, required this.session});

  final Session session;

  @override
  State<DesksharingApp> createState() => _DesksharingAppState();
}

class _DesksharingAppState extends State<DesksharingApp> {
  @override
  void initState() {
    super.initState();
    widget.session.boot();
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.session,
      builder: (context, _) {
        final session = widget.session;
        return MaterialApp(
          title: 'Desksharing',
          theme: buildTheme(),
          debugShowCheckedModeBanner: false,
          home: session.booting
              ? const _BootScreen()
              : session.bootError != null &&
                    session.user == null &&
                    session.token != null
              ? _OfflineScreen(session: session)
              : session.user == null
              ? LoginScreen(session: session)
              : ShellScreen(session: session),
        );
      },
    );
  }
}

class _BootScreen extends StatelessWidget {
  const _BootScreen();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(body: Center(child: CircularProgressIndicator()));
  }
}

class _OfflineScreen extends StatelessWidget {
  const _OfflineScreen({required this.session});

  final Session session;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.cloud_off, size: 42, color: accent),
                const SizedBox(height: 12),
                Text(
                  session.bootError ?? 'Server nicht erreichbar.',
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: session.boot,
                  child: const Text('Erneut versuchen'),
                ),
                TextButton(
                  onPressed: session.expire,
                  child: const Text('Abmelden'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
