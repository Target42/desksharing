import 'package:flutter/material.dart';

import '../api.dart';
import '../format.dart';
import '../models.dart';
import '../session.dart';
import '../theme.dart';
import '../widgets/choice_field.dart';
import '../widgets/error_banner.dart';

class ResourcesScreen extends StatefulWidget {
  const ResourcesScreen({super.key, required this.session});

  final Session session;

  @override
  State<ResourcesScreen> createState() => _ResourcesScreenState();
}

class _ResourcesScreenState extends State<ResourcesScreen> {
  bool _loading = true;
  bool _searching = false;
  String? _bookingId;
  String? _error;

  List<ResourceGroup> _groups = const [];
  List<LocationItem> _locations = const [];
  String? _groupId;
  String? _locationId;
  String _mode = 'hourly';

  late DateTime _day;
  late String _start;
  late String _end;
  late DateTime _startDate;
  late DateTime _endDate;
  bool _onlyAvailable = true;
  List<BookableResource> _results = const [];
  bool _searched = false;

  @override
  void initState() {
    super.initState();
    final slot = defaultResourceHourly(DateTime.now());
    _day = slot.day;
    _start = slot.start;
    _end = slot.end;
    final today = DateTime.now();
    _startDate = DateTime(today.year, today.month, today.day);
    _endDate = _startDate;
    _loadFilters();
  }

  Future<void> _loadFilters() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = widget.session.api();
      final results = await Future.wait([api.resourceGroups(), api.locations()]);
      if (!mounted) return;
      setState(() {
        _groups = results[0] as List<ResourceGroup>;
        _locations = results[1] as List<LocationItem>;
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

  Future<void> _pickGroup() async {
    final id = await showChoiceSheet(
      context,
      title: 'Gruppe',
      selectedId: _groupId ?? '',
      options: [
        const ChoiceOption(id: '', label: 'alle'),
        ..._groups.map((item) => ChoiceOption(id: item.id, label: item.name)),
      ],
    );
    if (id == null || !mounted) return;
    setState(() => _groupId = id.isEmpty ? null : id);
  }

  Future<void> _pickLocation() async {
    final id = await showChoiceSheet(
      context,
      title: 'Standort',
      selectedId: _locationId ?? '',
      options: [
        const ChoiceOption(id: '', label: 'alle'),
        ..._locations.map(
          (item) => ChoiceOption(id: item.id, label: item.name),
        ),
      ],
    );
    if (id == null || !mounted) return;
    setState(() => _locationId = id.isEmpty ? null : id);
  }

  DateTime get _today {
    final now = DateTime.now();
    return DateTime(now.year, now.month, now.day);
  }

  DateTime get _lastDay => _today.add(const Duration(days: 366));

  Future<void> _pickDay() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _day.isBefore(_today) ? _today : _day,
      firstDate: _today,
      lastDate: _lastDay,
    );
    if (picked == null || !mounted) return;
    setState(() => _day = DateTime(picked.year, picked.month, picked.day));
  }

  Future<void> _pickRange({required bool end}) async {
    final initial = end ? _endDate : _startDate;
    final first = end ? _startDate : _today;
    final picked = await showDatePicker(
      context: context,
      initialDate: initial.isBefore(first) ? first : initial,
      firstDate: first,
      lastDate: _lastDay,
    );
    if (picked == null || !mounted) return;
    final day = DateTime(picked.year, picked.month, picked.day);
    setState(() {
      if (end) {
        _endDate = day;
      } else {
        _startDate = day;
        if (_endDate.isBefore(_startDate)) _endDate = _startDate;
      }
    });
  }

  Future<void> _pickClock({required bool end}) async {
    final current = end ? _end : _start;
    final id = await showChoiceSheet(
      context,
      title: end ? 'Bis' : 'Von',
      selectedId: current,
      options: [
        for (var minutes = 0; minutes < 24 * 60; minutes += 15)
          ChoiceOption(id: _hhmm(minutes), label: _hhmm(minutes)),
      ],
    );
    if (id == null || !mounted) return;
    setState(() {
      if (end) {
        _end = id;
      } else {
        _start = id;
        if (_clockMinutes(_end) <= _clockMinutes(_start)) {
          final next = _clockMinutes(_start) + 60;
          _end = next >= 24 * 60 ? '23:45' : _hhmm(next);
        }
      }
    });
  }

  String? _windowError() {
    if (_mode == 'daily') {
      if (_endDate.isBefore(_startDate)) {
        return 'Enddatum liegt vor dem Startdatum.';
      }
      final endExclusive = _endDate.add(const Duration(days: 1));
      if (!endExclusive.isAfter(DateTime.now())) {
        return 'Dieser Zeitraum liegt in der Vergangenheit.';
      }
      return null;
    }
    final startMin = _clockMinutes(_start);
    final endMin = _clockMinutes(_end);
    if (endMin <= startMin) {
      return 'Bitte gültigen Zeitraum wählen (Bis nach Von).';
    }
    final endsAt = DateTime(
      _day.year,
      _day.month,
      _day.day,
      endMin ~/ 60,
      endMin % 60,
    );
    if (!endsAt.isAfter(DateTime.now())) {
      return 'Dieser Zeitraum liegt in der Vergangenheit.';
    }
    return null;
  }

  String _iso(int minutes) {
    final value = DateTime(
      _day.year,
      _day.month,
      _day.day,
      minutes ~/ 60,
      minutes % 60,
    );
    return value.toUtc().toIso8601String();
  }

  Future<void> _search() async {
    final problem = _windowError();
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    setState(() {
      _searching = true;
      _error = null;
    });
    try {
      final items = await widget.session.api().searchResources(
        mode: _mode,
        groupId: _groupId,
        locationId: _locationId,
        startsAt: _mode == 'hourly' ? _iso(_clockMinutes(_start)) : '',
        endsAt: _mode == 'hourly' ? _iso(_clockMinutes(_end)) : '',
        startDate: _mode == 'daily' ? ymd(_startDate) : '',
        endDate: _mode == 'daily' ? ymd(_endDate) : '',
        onlyAvailable: _onlyAvailable,
      );
      if (!mounted) return;
      setState(() {
        _results = items;
        _searched = true;
        _searching = false;
      });
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        await widget.session.expire();
        return;
      }
      if (!mounted) return;
      setState(() {
        _searching = false;
        _error = error.message;
      });
    }
  }

  Future<void> _book(BookableResource item) async {
    final problem = _windowError();
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    setState(() {
      _bookingId = item.id;
      _error = null;
    });
    try {
      await widget.session.api().createResourceBooking(
        resourceId: item.id,
        mode: _mode,
        startsAt: _mode == 'hourly' ? _iso(_clockMinutes(_start)) : '',
        endsAt: _mode == 'hourly' ? _iso(_clockMinutes(_end)) : '',
        startDate: _mode == 'daily' ? ymd(_startDate) : '',
        endDate: _mode == 'daily' ? ymd(_endDate) : '',
      );
      if (!mounted) return;
      widget.session.bumpBookings();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('${item.name} gebucht.')),
      );
      setState(() => _bookingId = null);
      await _search();
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        await widget.session.expire();
        return;
      }
      if (!mounted) return;
      setState(() {
        _bookingId = null;
        _error = error.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    final groupLabel = _groupId == null
        ? 'alle'
        : _groups
              .where((item) => item.id == _groupId)
              .map((item) => item.name)
              .firstOrNull;
    final locationLabel = _locationId == null
        ? 'alle'
        : _locations
              .where((item) => item.id == _locationId)
              .map((item) => item.name)
              .firstOrNull;

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 28),
      children: [
        Text(
          'Stundenweise am selben Tag oder für einen bis mehrere Tage.',
          style: TextStyle(color: ink.withValues(alpha: 0.7)),
        ),
        const SizedBox(height: 12),
        if (_error != null)
          ErrorBanner(message: _error!, onRetry: _searching ? null : _search),
        ChoiceField(
          label: 'Gruppe',
          value: groupLabel ?? 'alle',
          enabled: true,
          onTap: _pickGroup,
        ),
        ChoiceField(
          label: 'Standort',
          value: locationLabel ?? 'alle',
          enabled: true,
          onTap: _pickLocation,
        ),
        SegmentedButton<String>(
          segments: const [
            ButtonSegment(value: 'hourly', label: Text('Stunden')),
            ButtonSegment(value: 'daily', label: Text('Tage')),
          ],
          selected: {_mode},
          onSelectionChanged: (value) => setState(() => _mode = value.first),
        ),
        const SizedBox(height: 12),
        if (_mode == 'hourly') ...[
          ChoiceField(
            label: 'Tag',
            value: formatLongDate(ymd(_day)),
            enabled: true,
            onTap: _pickDay,
          ),
          Row(
            children: [
              Expanded(
                child: ChoiceField(
                  label: 'Von',
                  value: _start,
                  enabled: true,
                  onTap: () => _pickClock(end: false),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: ChoiceField(
                  label: 'Bis',
                  value: _end,
                  enabled: true,
                  onTap: () => _pickClock(end: true),
                ),
              ),
            ],
          ),
        ] else ...[
          ChoiceField(
            label: 'Von',
            value: formatLongDate(ymd(_startDate)),
            enabled: true,
            onTap: () => _pickRange(end: false),
          ),
          ChoiceField(
            label: 'Bis (inklusive)',
            value: formatLongDate(ymd(_endDate)),
            enabled: true,
            onTap: () => _pickRange(end: true),
          ),
        ],
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('Nur freie Ressourcen'),
          value: _onlyAvailable,
          onChanged: (value) => setState(() => _onlyAvailable = value),
        ),
        FilledButton(
          onPressed: _searching || _bookingId != null ? null : _search,
          child: _searching
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Text('Verfügbarkeit prüfen'),
        ),
        const SizedBox(height: 18),
        Text('Treffer', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        if (!_searched)
          Text(
            'Zeitraum wählen und die Verfügbarkeit prüfen.',
            style: TextStyle(color: ink.withValues(alpha: 0.7)),
          )
        else if (_results.isEmpty)
          Text(
            'Keine Ressourcen in diesem Zeitraum.',
            style: TextStyle(color: ink.withValues(alpha: 0.7)),
          )
        else
          ..._results.map(_resultCard),
      ],
    );
  }

  Widget _resultCard(BookableResource item) {
    final busy = _bookingId == item.id;
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(14, 12, 14, 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              item.name,
              style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
            ),
            if (item.subtitle.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                '${item.available ? 'frei' : 'belegt'} · ${item.subtitle}',
                style: TextStyle(color: ink.withValues(alpha: 0.7)),
              ),
            ],
            if (item.available)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: _bookingId != null || _searching
                      ? null
                      : () => _book(item),
                  child: busy
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Buchen'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

int _clockMinutes(String hhmm) {
  final parts = hhmm.split(':');
  if (parts.length != 2) return 0;
  final hour = int.tryParse(parts[0]) ?? 0;
  final minute = int.tryParse(parts[1]) ?? 0;
  return hour * 60 + minute;
}

String _hhmm(int minutes) {
  final hour = minutes ~/ 60;
  final minute = minutes % 60;
  return '${twoDigits(hour)}:${twoDigits(minute)}';
}
