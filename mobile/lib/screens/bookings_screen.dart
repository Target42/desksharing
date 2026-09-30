import 'package:flutter/material.dart';

import '../api.dart';
import '../format.dart';
import '../models.dart';
import '../session.dart';
import '../theme.dart';
import '../widgets/error_banner.dart';

class BookingsScreen extends StatefulWidget {
  const BookingsScreen({super.key, required this.session});

  final Session session;

  @override
  State<BookingsScreen> createState() => _BookingsScreenState();
}

class _BookingsScreenState extends State<BookingsScreen> {
  List<BookingItem> _items = const [];
  bool _loading = true;
  String? _error;
  String? _cancellingId;
  late int _seenTick;

  @override
  void initState() {
    super.initState();
    _seenTick = widget.session.bookingsTick;
    widget.session.addListener(_onSession);
    _load();
  }

  @override
  void dispose() {
    widget.session.removeListener(_onSession);
    super.dispose();
  }

  void _onSession() {
    if (!mounted) return;
    if (widget.session.bookingsTick != _seenTick) {
      _seenTick = widget.session.bookingsTick;
      _load();
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final items = await widget.session.api().myBookings();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        await widget.session.expire();
        return;
      }
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = error.message;
      });
    }
  }

  Future<void> _cancel(BookingItem item) async {
    final label = item.placeLabel.isEmpty ? 'Buchung' : item.placeLabel;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Buchung stornieren?'),
        content: Text(
          '$label\n${formatBookingRange(item.startsAt, item.endsAt)}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Abbrechen'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: danger),
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Stornieren'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _cancellingId = item.id);
    try {
      await widget.session.api().cancelBooking(item.id);
      if (!mounted) return;
      widget.session.bumpBookings();
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Buchung storniert.')));
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        await widget.session.expire();
        return;
      }
      if (!mounted) return;
      setState(() => _error = error.message);
    } finally {
      if (mounted) setState(() => _cancellingId = null);
    }
  }

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          if (_error != null) ErrorBanner(message: _error!, onRetry: _load),
          if (_loading && _items.isEmpty)
            const Padding(
              padding: EdgeInsets.only(top: 48),
              child: Center(child: CircularProgressIndicator()),
            )
          else if (_items.isEmpty)
            const Padding(
              padding: EdgeInsets.only(top: 48),
              child: Column(
                children: [
                  Icon(Icons.event_busy_outlined, size: 42, color: accent),
                  SizedBox(height: 12),
                  Text('Keine anstehenden Buchungen.'),
                ],
              ),
            )
          else
            for (final item in _items)
              _BookingCard(
                item: item,
                busy: _cancellingId == item.id,
                onCancel: item.canCancel ? () => _cancel(item) : null,
              ),
        ],
      ),
    );
  }
}

class _BookingCard extends StatelessWidget {
  const _BookingCard({
    required this.item,
    required this.busy,
    required this.onCancel,
  });

  final BookingItem item;
  final bool busy;
  final VoidCallback? onCancel;

  @override
  Widget build(BuildContext context) {
    final title = item.placeLabel.isEmpty ? 'Buchung' : item.placeLabel;
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(14, 12, 14, 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    title,
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 16,
                    ),
                  ),
                ),
                _StatusChip(status: item.status),
              ],
            ),
            const SizedBox(height: 6),
            Text(formatBookingRange(item.startsAt, item.endsAt)),
            if (item.bookingMode.isNotEmpty)
              Text(
                bookingModeLabel(item.bookingMode),
                style: TextStyle(
                  color: ink.withValues(alpha: 0.62),
                  fontSize: 13,
                ),
              ),
            if (onCancel != null)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: busy ? null : onCancel,
                  child: busy
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Stornieren'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    final warn = status == 'possibly_free';
    final color = warn ? const Color(0xFF8A5A00) : accentDark;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        bookingStatusLabel(status),
        style: TextStyle(
          color: color,
          fontSize: 12,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
