import 'package:flutter/material.dart';

import '../api.dart';
import '../format.dart';
import '../models.dart';
import '../session.dart';
import '../theme.dart';
import '../widgets/error_banner.dart';

class BookDaySheet extends StatefulWidget {
  const BookDaySheet({
    super.key,
    required this.session,
    required this.placeId,
    required this.placeLabel,
    required this.date,
    required this.bookingMode,
    required this.dayStatus,
  });

  final Session session;
  final String placeId;
  final String placeLabel;
  final String date;
  final String bookingMode;
  final String dayStatus;

  @override
  State<BookDaySheet> createState() => _BookDaySheetState();
}

class _BookDaySheetState extends State<BookDaySheet> {
  PlaceDayDetail? _detail;
  bool _loading = true;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final detail = await widget.session.api().placeDay(
        placeId: widget.placeId,
        date: widget.date,
      );
      if (!mounted) return;
      setState(() {
        _detail = detail;
        _loading = false;
      });
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        if (mounted) Navigator.pop(context, false);
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

  Future<void> _book(TimeSlot slot) async {
    if (slot.startsAt.isEmpty || slot.endsAt.isEmpty) {
      setState(() => _error = 'Dieser Zeitraum kann nicht gebucht werden.');
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await widget.session.api().createBooking(
        placeId: widget.placeId,
        startsAt: slot.startsAt,
        endsAt: slot.endsAt,
      );
      if (mounted) Navigator.pop(context, true);
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        if (mounted) Navigator.pop(context, false);
        await widget.session.expire();
        return;
      }
      if (!mounted) return;
      setState(() {
        _saving = false;
        _error = error.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final detail = _detail;
    final fullDay = widget.dayStatus == 'free' ? detail?.fullDay : null;
    final partials = (detail?.free ?? const <TimeSlot>[])
        .where((slot) => !slot.isFullDay)
        .toList();
    final hourly = widget.bookingMode == 'hourly';

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 0, 20, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              widget.placeLabel,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 4),
            Text(
              '${formatLongDate(widget.date)} · ${bookingModeLabel(widget.bookingMode)}',
              style: TextStyle(color: ink.withValues(alpha: 0.7)),
            ),
            const SizedBox(height: 14),
            if (_error != null)
              ErrorBanner(message: _error!, onRetry: _loading ? null : _load),
            if (_loading)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 28),
                child: Center(child: CircularProgressIndicator()),
              )
            else if (detail != null) ...[
              if (detail.busy.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: Text(
                    'Belegt: ${detail.busy.map((slot) => '${slot.start}–${slot.end}').join(', ')}',
                  ),
                ),
              if (detail.free.isEmpty)
                const Text('Keine freien Zeiten an diesem Tag.')
              else ...[
                if (fullDay != null && !hourly)
                  FilledButton(
                    onPressed: _saving ? null : () => _book(fullDay),
                    child: const Text('Ganzen Tag buchen (08:00–18:00)'),
                  ),
                if (partials.isNotEmpty) ...[
                  Padding(
                    padding: EdgeInsets.only(
                      top: fullDay != null && !hourly ? 16 : 0,
                      bottom: 8,
                    ),
                    child: Text(
                      fullDay != null && !hourly
                          ? 'Oder einen Zeitraum'
                          : 'Freie Zeiten',
                      style: const TextStyle(fontWeight: FontWeight.w600),
                    ),
                  ),
                  for (final slot in partials)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: OutlinedButton(
                        onPressed: _saving ? null : () => _book(slot),
                        child: Text('${slot.start}–${slot.end}'),
                      ),
                    ),
                ],
                if (fullDay != null && hourly)
                  partials.isEmpty
                      ? FilledButton(
                          onPressed: _saving ? null : () => _book(fullDay),
                          child: const Text('Ganzen Tag buchen (08:00–18:00)'),
                        )
                      : TextButton(
                          onPressed: _saving ? null : () => _book(fullDay),
                          child: const Text(
                            'Stattdessen ganzen Tag (08:00–18:00)',
                          ),
                        ),
              ],
              if (_saving)
                const Padding(
                  padding: EdgeInsets.only(top: 8),
                  child: Center(child: CircularProgressIndicator()),
                ),
            ],
          ],
        ),
      ),
    );
  }
}
