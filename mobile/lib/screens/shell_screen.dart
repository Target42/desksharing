import 'package:flutter/material.dart';

import '../session.dart';
import 'book_screen.dart';
import 'bookings_screen.dart';
import 'resources_screen.dart';

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
        leadingWidth: 56,
        leading: Padding(
          padding: const EdgeInsets.all(10),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: Image.asset(
              'assets/logo.png',
              width: 36,
              height: 36,
              fit: BoxFit.cover,
            ),
          ),
        ),
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              switch (_index) {
                0 => 'Platz buchen',
                1 => 'Ressource buchen',
                _ => 'Meine Buchungen',
              },
            ),
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
          ResourcesScreen(session: widget.session),
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
            icon: Icon(Icons.inventory_2_outlined),
            selectedIcon: Icon(Icons.inventory_2),
            label: 'Ressourcen',
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
