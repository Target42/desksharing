DateTime? parseYmd(String raw) {
  final parts = raw.split('-');
  if (parts.length != 3) return null;
  final year = int.tryParse(parts[0]);
  final month = int.tryParse(parts[1]);
  final day = int.tryParse(parts[2]);
  if (year == null || month == null || day == null) return null;
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  return DateTime(year, month, day);
}

DateTime mondayOf(DateTime date) {
  final day = DateTime(date.year, date.month, date.day);
  return day.subtract(Duration(days: day.weekday - DateTime.monday));
}

String ymd(DateTime date) {
  final month = date.month.toString().padLeft(2, '0');
  final day = date.day.toString().padLeft(2, '0');
  return '${date.year.toString().padLeft(4, '0')}-$month-$day';
}

String twoDigits(int value) => value.toString().padLeft(2, '0');

const weekdayShort = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'];
const weekdayLong = [
  'Montag',
  'Dienstag',
  'Mittwoch',
  'Donnerstag',
  'Freitag',
  'Samstag',
  'Sonntag',
];
const monthShort = [
  'Jan.',
  'Feb.',
  'März',
  'Apr.',
  'Mai',
  'Juni',
  'Juli',
  'Aug.',
  'Sep.',
  'Okt.',
  'Nov.',
  'Dez.',
];

String formatWeekSpan(String start, String end) {
  final a = parseYmd(start);
  final b = parseYmd(end);
  if (a == null || b == null) return '$start – $end';
  return '${twoDigits(a.day)}. ${monthShort[a.month - 1]} – ${twoDigits(b.day)}. ${monthShort[b.month - 1]}';
}

String formatLongDate(String raw) {
  final date = parseYmd(raw);
  if (date == null) return raw;
  return '${weekdayLong[date.weekday - 1]}, ${twoDigits(date.day)}.${twoDigits(date.month)}.${date.year}';
}

String formatClock(DateTime value) {
  final local = value.toLocal();
  return '${twoDigits(local.hour)}:${twoDigits(local.minute)}';
}

String formatDateTimeDe(DateTime value) {
  final local = value.toLocal();
  return '${twoDigits(local.day)}.${twoDigits(local.month)}.${local.year} ${formatClock(local)}';
}

bool _isMidnight(DateTime value) =>
    value.hour == 0 && value.minute == 0 && value.second == 0;

String _shortDate(DateTime value) =>
    '${twoDigits(value.day)}.${twoDigits(value.month)}.${value.year}';

String formatBookingRange(DateTime start, DateTime end) {
  final a = start.toLocal();
  final b = end.toLocal();
  if (_isMidnight(a) && _isMidnight(b) && b.isAfter(a)) {
    final last = b.subtract(const Duration(minutes: 1));
    final from = _shortDate(a);
    final to = _shortDate(last);
    return from == to ? from : '$from – $to';
  }
  final sameDay = a.year == b.year && a.month == b.month && a.day == b.day;
  if (sameDay) {
    final date = DateTime(a.year, a.month, a.day);
    return '${weekdayLong[date.weekday - 1]}, ${twoDigits(a.day)}.${twoDigits(a.month)}. · ${twoDigits(a.hour)}:${twoDigits(a.minute)}–${twoDigits(b.hour)}:${twoDigits(b.minute)}';
  }
  return '${formatDateTimeDe(a)} – ${formatDateTimeDe(b)}';
}

class ResourceHourlyDefault {
  const ResourceHourlyDefault({
    required this.day,
    required this.start,
    required this.end,
  });

  final DateTime day;
  final String start;
  final String end;
}

/// Next bookable hourly window: the coming quarter hour plus two hours.
/// After 22:00 the suggestion is the next morning, 09:00–12:00.
ResourceHourlyDefault defaultResourceHourly(DateTime now) {
  var start = DateTime(now.year, now.month, now.day, now.hour, now.minute);
  final remainder = start.minute % 15;
  if (remainder != 0) {
    start = start.add(Duration(minutes: 15 - remainder));
  }
  if (!start.isAfter(now)) {
    start = start.add(const Duration(minutes: 15));
  }
  final end = start.add(const Duration(hours: 2));
  final sameDay =
      end.year == start.year &&
      end.month == start.month &&
      end.day == start.day;
  if (!sameDay || start.hour >= 22) {
    final tomorrow = DateTime(
      now.year,
      now.month,
      now.day,
    ).add(const Duration(days: 1));
    return ResourceHourlyDefault(day: tomorrow, start: '09:00', end: '12:00');
  }
  return ResourceHourlyDefault(
    day: DateTime(start.year, start.month, start.day),
    start: formatClock(start),
    end: formatClock(end),
  );
}

String bookingStatusLabel(String status) {
  return switch (status) {
    'booked' => 'gebucht',
    'checked_in' => 'eingecheckt',
    'checked_out' => 'ausgecheckt',
    'cancelled' => 'storniert',
    'possibly_free' => 'möglicherweise frei',
    'no_show' => 'No-Show',
    _ => status,
  };
}

String bookingModeLabel(String mode) {
  return switch (mode) {
    'daily' => 'tageweise',
    'hourly' => 'stundenweise',
    _ => mode,
  };
}

String dayStatusLabel(String status) {
  return switch (status) {
    'free' => 'frei',
    'partial' => 'teilweise',
    'full' => 'belegt',
    'blocked' => 'gesperrt',
    _ => status,
  };
}

bool dayIsBookable(String status) => status == 'free' || status == 'partial';
