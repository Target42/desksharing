import 'package:flutter/material.dart';

import '../api.dart';
import '../format.dart';
import '../models.dart';
import '../session.dart';
import '../theme.dart';
import '../widgets/choice_field.dart';
import '../widgets/error_banner.dart';
import 'book_sheet.dart';

class BookScreen extends StatefulWidget {
  const BookScreen({super.key, required this.session});

  final Session session;

  @override
  State<BookScreen> createState() => _BookScreenState();
}

class _BookScreenState extends State<BookScreen> {
  static const _kLocation = 'book.locationId';
  static const _kBuilding = 'book.buildingId';
  static const _kFloor = 'book.floorId';
  static const _kRoom = 'book.roomId';

  int _gen = 0;
  int _weekGen = 0;
  bool _loadingFilters = true;
  bool _loadingWeek = false;
  String? _filterError;
  String? _weekError;

  List<LocationItem> _locations = const [];
  List<BuildingItem> _buildings = const [];
  List<FloorItem> _floors = const [];
  List<RoomItem> _rooms = const [];
  String? _locationId;
  String? _buildingId;
  String? _floorId;
  String? _roomId;
  RoomWeek? _week;
  late String _weekStart;

  @override
  void initState() {
    super.initState();
    _weekStart = ymd(mondayOf(DateTime.now()));
    _loadInitial();
  }

  bool _alive(int gen) => mounted && gen == _gen;

  Future<void> _loadInitial() async {
    final gen = ++_gen;
    setState(() {
      _loadingFilters = true;
      _filterError = null;
    });
    try {
      final locations = await widget.session.api().locations();
      if (!_alive(gen)) return;
      final locationId = preferId(
        widget.session.prefs.getString(_kLocation),
        locations.map((item) => item.id).toList(),
      );
      setState(() {
        _locations = locations;
        _locationId = locationId;
      });
      if (locationId != null) {
        await widget.session.prefs.setString(_kLocation, locationId);
        await _loadBuildings(gen, restore: true);
        return;
      }
      if (_alive(gen)) setState(() => _loadingFilters = false);
    } on ApiException catch (error) {
      await _failFilters(gen, error);
    }
  }

  Future<void> _onLocation(String id) async {
    final gen = ++_gen;
    setState(() {
      _locationId = id;
      _buildings = const [];
      _buildingId = null;
      _floors = const [];
      _floorId = null;
      _rooms = const [];
      _roomId = null;
      _week = null;
      _weekError = null;
      _loadingFilters = true;
      _filterError = null;
    });
    await widget.session.prefs.setString(_kLocation, id);
    await _loadBuildings(gen, restore: false);
  }

  Future<void> _loadBuildings(int gen, {required bool restore}) async {
    final locationId = _locationId;
    if (locationId == null) {
      if (_alive(gen)) setState(() => _loadingFilters = false);
      return;
    }
    try {
      final buildings = await widget.session.api().buildings(locationId);
      if (!_alive(gen)) return;
      final buildingId = preferId(
        restore ? widget.session.prefs.getString(_kBuilding) : null,
        buildings.map((item) => item.id).toList(),
      );
      setState(() {
        _buildings = buildings;
        _buildingId = buildingId;
      });
      if (buildingId != null) {
        await widget.session.prefs.setString(_kBuilding, buildingId);
        await _loadFloors(gen, restore: restore);
        return;
      }
      if (_alive(gen)) setState(() => _loadingFilters = false);
    } on ApiException catch (error) {
      await _failFilters(gen, error);
    }
  }

  Future<void> _onBuilding(String id) async {
    final gen = ++_gen;
    setState(() {
      _buildingId = id;
      _floors = const [];
      _floorId = null;
      _rooms = const [];
      _roomId = null;
      _week = null;
      _weekError = null;
      _loadingFilters = true;
    });
    await widget.session.prefs.setString(_kBuilding, id);
    await _loadFloors(gen, restore: false);
  }

  Future<void> _loadFloors(int gen, {required bool restore}) async {
    final buildingId = _buildingId;
    if (buildingId == null) {
      if (_alive(gen)) setState(() => _loadingFilters = false);
      return;
    }
    try {
      final floors = await widget.session.api().floors(buildingId);
      if (!_alive(gen)) return;
      final floorId = preferId(
        restore ? widget.session.prefs.getString(_kFloor) : null,
        floors.map((item) => item.id).toList(),
      );
      setState(() {
        _floors = floors;
        _floorId = floorId;
      });
      if (floorId != null) {
        await widget.session.prefs.setString(_kFloor, floorId);
        await _loadRooms(gen, restore: restore);
        return;
      }
      if (_alive(gen)) setState(() => _loadingFilters = false);
    } on ApiException catch (error) {
      await _failFilters(gen, error);
    }
  }

  Future<void> _onFloor(String id) async {
    final gen = ++_gen;
    setState(() {
      _floorId = id;
      _rooms = const [];
      _roomId = null;
      _week = null;
      _weekError = null;
      _loadingFilters = true;
    });
    await widget.session.prefs.setString(_kFloor, id);
    await _loadRooms(gen, restore: false);
  }

  Future<void> _loadRooms(int gen, {required bool restore}) async {
    final floorId = _floorId;
    if (floorId == null) {
      if (_alive(gen)) setState(() => _loadingFilters = false);
      return;
    }
    try {
      final rooms = await widget.session.api().rooms(floorId);
      if (!_alive(gen)) return;
      final roomId = preferId(
        restore ? widget.session.prefs.getString(_kRoom) : null,
        rooms.map((item) => item.id).toList(),
      );
      setState(() {
        _rooms = rooms;
        _roomId = roomId;
        _loadingFilters = false;
      });
      if (roomId != null) {
        await widget.session.prefs.setString(_kRoom, roomId);
        await _loadWeek();
      }
    } on ApiException catch (error) {
      await _failFilters(gen, error);
    }
  }

  Future<void> _onRoom(String id) async {
    setState(() {
      _roomId = id;
      _week = null;
      _weekError = null;
    });
    await widget.session.prefs.setString(_kRoom, id);
    await _loadWeek();
  }

  Future<void> _failFilters(int gen, ApiException error) async {
    if (error.isUnauthorized) {
      await widget.session.expire();
      return;
    }
    if (!_alive(gen)) return;
    setState(() {
      _loadingFilters = false;
      _filterError = error.message;
    });
  }

  Future<void> _loadWeek() async {
    final roomId = _roomId;
    final gen = ++_weekGen;
    if (roomId == null) {
      if (mounted) {
        setState(() {
          _week = null;
          _loadingWeek = false;
        });
      }
      return;
    }
    setState(() {
      _loadingWeek = true;
      _weekError = null;
    });
    try {
      final week = await widget.session.api().roomWeek(
        roomId: roomId,
        weekStart: _weekStart,
      );
      if (!mounted || gen != _weekGen) return;
      setState(() {
        _week = week;
        if (week.weekStart.isNotEmpty) _weekStart = week.weekStart;
        _loadingWeek = false;
      });
    } on ApiException catch (error) {
      if (error.isUnauthorized) {
        await widget.session.expire();
        return;
      }
      if (!mounted || gen != _weekGen) return;
      setState(() {
        _loadingWeek = false;
        _weekError = error.message;
      });
    }
  }

  Future<void> _shiftWeek(int days) async {
    final current = parseYmd(_weekStart) ?? mondayOf(DateTime.now());
    setState(() => _weekStart = ymd(current.add(Duration(days: days))));
    await _loadWeek();
  }

  Future<void> _refresh() async {
    if (_roomId != null) {
      await _loadWeek();
    } else {
      await _loadInitial();
    }
  }

  Future<void> _openDay(PlaceWeek place, PlaceDay day) async {
    if (!dayIsBookable(day.status)) return;
    final booked = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (context) => BookDaySheet(
        session: widget.session,
        placeId: place.id,
        placeLabel: place.label,
        date: day.date,
        bookingMode: place.bookingMode,
        dayStatus: day.status,
      ),
    );
    if (booked != true || !mounted) return;
    widget.session.bumpBookings();
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text('${place.label} gebucht.')));
    await _loadWeek();
  }

  Future<void> _pick(
    String title,
    List<ChoiceOption> options,
    String? selected,
    Future<void> Function(String id) onSelected,
  ) async {
    if (options.isEmpty) return;
    final id = await showChoiceSheet(
      context,
      title: title,
      options: options,
      selectedId: selected,
    );
    if (id != null) await onSelected(id);
  }

  String? _labelOf(String? id, List<({String id, String label})> items) {
    if (id == null) return null;
    for (final item in items) {
      if (item.id == id) return item.label;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final locationOptions = [
      for (final item in _locations)
        ChoiceOption(id: item.id, label: item.name),
    ];
    final buildingOptions = [
      for (final item in _buildings)
        ChoiceOption(id: item.id, label: item.label),
    ];
    final floorOptions = [
      for (final item in _floors) ChoiceOption(id: item.id, label: item.label),
    ];
    final roomOptions = [
      for (final item in _rooms) ChoiceOption(id: item.id, label: item.label),
    ];
    final week = _week;

    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          if (_filterError != null)
            ErrorBanner(message: _filterError!, onRetry: _loadInitial),
          if (_loadingFilters && _locations.isEmpty && _filterError == null)
            const Padding(
              padding: EdgeInsets.only(top: 48),
              child: Center(child: CircularProgressIndicator()),
            )
          else if (_locations.isEmpty && !_loadingFilters)
            const Padding(
              padding: EdgeInsets.only(top: 32),
              child: Text(
                'Es sind noch keine Plätze eingerichtet. Bitte wende dich an deine Administration.',
              ),
            )
          else ...[
            ChoiceField(
              label: 'Standort',
              value: _labelOf(_locationId, [
                for (final item in _locations) (id: item.id, label: item.name),
              ]),
              enabled: !_loadingFilters && locationOptions.isNotEmpty,
              onTap: () =>
                  _pick('Standort', locationOptions, _locationId, _onLocation),
            ),
            ChoiceField(
              label: 'Gebäude',
              value: _labelOf(_buildingId, [
                for (final item in _buildings) (id: item.id, label: item.label),
              ]),
              enabled:
                  !_loadingFilters &&
                  _locationId != null &&
                  buildingOptions.isNotEmpty,
              onTap: () =>
                  _pick('Gebäude', buildingOptions, _buildingId, _onBuilding),
            ),
            ChoiceField(
              label: 'Stockwerk',
              value: _labelOf(_floorId, [
                for (final item in _floors) (id: item.id, label: item.label),
              ]),
              enabled:
                  !_loadingFilters &&
                  _buildingId != null &&
                  floorOptions.isNotEmpty,
              onTap: () => _pick('Stockwerk', floorOptions, _floorId, _onFloor),
            ),
            ChoiceField(
              label: 'Raum',
              value: _labelOf(_roomId, [
                for (final item in _rooms) (id: item.id, label: item.label),
              ]),
              enabled:
                  !_loadingFilters &&
                  _floorId != null &&
                  roomOptions.isNotEmpty,
              onTap: () => _pick('Raum', roomOptions, _roomId, _onRoom),
            ),
            if (_loadingFilters) const LinearProgressIndicator(),
          ],
          if (_roomId == null && !_loadingFilters && _locations.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(
                'Bitte Standort, Gebäude, Stockwerk und Raum wählen.',
                style: TextStyle(color: ink.withValues(alpha: 0.7)),
              ),
            ),
          if (_roomId != null) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                IconButton(
                  tooltip: 'Vorherige Woche',
                  onPressed: _loadingWeek ? null : () => _shiftWeek(-7),
                  icon: const Icon(Icons.chevron_left),
                ),
                Expanded(
                  child: Text(
                    week == null
                        ? 'Woche'
                        : formatWeekSpan(week.weekStart, week.weekEnd),
                    textAlign: TextAlign.center,
                    style: const TextStyle(fontWeight: FontWeight.w700),
                  ),
                ),
                IconButton(
                  tooltip: 'Nächste Woche',
                  onPressed: _loadingWeek ? null : () => _shiftWeek(7),
                  icon: const Icon(Icons.chevron_right),
                ),
              ],
            ),
            if (week != null && week.roomLabel.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(week.roomLabel, textAlign: TextAlign.center),
              ),
            if (_loadingWeek) const LinearProgressIndicator(),
            if (_weekError != null)
              ErrorBanner(message: _weekError!, onRetry: _loadWeek),
            if (week != null && week.places.isEmpty && !_loadingWeek)
              const Padding(
                padding: EdgeInsets.only(top: 16),
                child: Text('Keine Plätze in diesem Raum.'),
              ),
            if (week != null)
              for (final place in week.places)
                _PlaceCard(place: place, onDay: (day) => _openDay(place, day)),
          ],
        ],
      ),
    );
  }
}

class _PlaceCard extends StatelessWidget {
  const _PlaceCard({required this.place, required this.onDay});

  final PlaceWeek place;
  final ValueChanged<PlaceDay> onDay;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              place.label,
              style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
            ),
            if (place.bookingMode.isNotEmpty)
              Text(
                bookingModeLabel(place.bookingMode),
                style: TextStyle(
                  color: ink.withValues(alpha: 0.62),
                  fontSize: 13,
                ),
              ),
            const SizedBox(height: 10),
            _DayGrid(days: place.days, onDay: onDay),
          ],
        ),
      ),
    );
  }
}

class _DayGrid extends StatelessWidget {
  const _DayGrid({required this.days, required this.onDay});

  final List<PlaceDay> days;
  final ValueChanged<PlaceDay> onDay;

  @override
  Widget build(BuildContext context) {
    const columns = 4;
    final rows = <List<PlaceDay?>>[];
    for (var i = 0; i < days.length; i += columns) {
      final slice = days.skip(i).take(columns).toList();
      rows.add([
        for (final day in slice) day,
        for (var pad = slice.length; pad < columns; pad++) null,
      ]);
    }
    return Column(
      children: [
        for (var row = 0; row < rows.length; row++)
          Padding(
            padding: EdgeInsets.only(top: row == 0 ? 0 : 8),
            child: Row(
              children: [
                for (var column = 0; column < columns; column++)
                  Expanded(
                    child: Padding(
                      padding: EdgeInsets.only(left: column == 0 ? 0 : 6),
                      child: rows[row][column] == null
                          ? const SizedBox.shrink()
                          : _DayChip(
                              day: rows[row][column]!,
                              onTap: dayIsBookable(rows[row][column]!.status)
                                  ? () => onDay(rows[row][column]!)
                                  : null,
                            ),
                    ),
                  ),
              ],
            ),
          ),
      ],
    );
  }
}

class _DayChip extends StatelessWidget {
  const _DayChip({required this.day, required this.onTap});

  final PlaceDay day;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final date = parseYmd(day.date);
    final weekday = date == null ? '' : weekdayShort[date.weekday - 1];
    final dayMonth = date == null
        ? day.date
        : '${twoDigits(date.day)}.${twoDigits(date.month)}.';
    final tone = _tone(day.status);
    final bookable = onTap != null;
    return Semantics(
      button: bookable,
      label: '$weekday $dayMonth, ${dayStatusLabel(day.status)}',
      child: Material(
        color: tone.background,
        borderRadius: BorderRadius.circular(12),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(12),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(4, 8, 4, 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  weekday,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: tone.foreground,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                Text(
                  dayMonth,
                  textAlign: TextAlign.center,
                  style: TextStyle(color: tone.foreground, fontSize: 12),
                ),
                const SizedBox(height: 4),
                FittedBox(
                  fit: BoxFit.scaleDown,
                  child: Text(
                    dayStatusLabel(day.status),
                    maxLines: 1,
                    style: TextStyle(color: tone.foreground, fontSize: 11),
                  ),
                ),
                const SizedBox(height: 6),
                _OccupancyBar(day: day),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _DayTone {
  const _DayTone(this.background, this.foreground);
  final Color background;
  final Color foreground;
}

_DayTone _tone(String status) {
  return switch (status) {
    'free' => const _DayTone(Color(0xFFDCEFE6), ok),
    'partial' => const _DayTone(Color(0xFFF8E8C8), Color(0xFF8A5A00)),
    'full' => const _DayTone(Color(0xFFF6D6D6), danger),
    _ => _DayTone(ink.withValues(alpha: 0.08), ink.withValues(alpha: 0.55)),
  };
}

class _OccupancyBar extends StatelessWidget {
  const _OccupancyBar({required this.day});

  final PlaceDay day;

  @override
  Widget build(BuildContext context) {
    final blocked = day.status == 'blocked' || day.status == 'full';
    return ClipRRect(
      borderRadius: BorderRadius.circular(99),
      child: SizedBox(
        height: 4,
        width: double.infinity,
        child: blocked || day.status == 'free' || day.segments.isEmpty
            ? ColoredBox(color: blocked ? danger : ok)
            : LayoutBuilder(
                builder: (context, constraints) {
                  final width = constraints.maxWidth;
                  return Stack(
                    children: [
                      const ColoredBox(
                        color: Color(0xFFDCEFE6),
                        child: SizedBox.expand(),
                      ),
                      for (final segment in day.segments)
                        Positioned(
                          left: width * (segment.startPct / 100).clamp(0, 1),
                          width:
                              width *
                              ((segment.endPct - segment.startPct) / 100).clamp(
                                0,
                                1,
                              ),
                          top: 0,
                          bottom: 0,
                          child: ColoredBox(
                            color: segment.kind == 'busy' ? danger : ok,
                          ),
                        ),
                    ],
                  );
                },
              ),
      ),
    );
  }
}
