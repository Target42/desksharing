import 'package:flutter/material.dart';

import '../session.dart';
import 'book_screen.dart';
import 'bookings_screen.dart';

class ShellScreen extends StatefulWidget {
  const ShellScreen({super.key, required this.session});

  final Session session;

  @override
  State<ShellScreen> createState() => _ShellScreenState();
}

class _ShellScreenState extends State<ShellScreen> {
  int _index = 0;

  Future<void> _logout() async {
    await widget.session.logout();
  }

  @override
  Widget build(BuildContext context) {
    final name = widget.session.user?.label ?? '';
    return Scaffold(
      appBar: AppBar(
        toolbarHeight: 68,
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(_index == 0 ? 'Platz buchen' : 'Meine Buchungen'),
            if (name.isNotEmpty)
              Text(
                name,
                style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w400,
                ),
              ),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'Abmelden',
            onPressed: _logout,
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: IndexedStack(
        index: _index,
        children: [
          BookScreen(session: widget.session),
          BookingsScreen(session: widget.session),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (index) => setState(() => _index = index),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.event_available_outlined),
            selectedIcon: Icon(Icons.event_available),
            label: 'Buchen',
          ),
          NavigationDestination(
            icon: Icon(Icons.list_alt_outlined),
            selectedIcon: Icon(Icons.list_alt),
            label: 'Buchungen',
          ),
        ],
      ),
    );
  }
}
