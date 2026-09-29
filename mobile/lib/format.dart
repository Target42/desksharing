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

String formatBookingRange(DateTime start, DateTime end) {
  final a = start.toLocal();
  final b = end.toLocal();
  final sameDay = a.year == b.year && a.month == b.month && a.day == b.day;
  if (sameDay) {
    final date = DateTime(a.year, a.month, a.day);
    return '${weekdayLong[date.weekday - 1]}, ${twoDigits(a.day)}.${twoDigits(a.month)}. · ${twoDigits(a.hour)}:${twoDigits(a.minute)}–${twoDigits(b.hour)}:${twoDigits(b.minute)}';
  }
  return '${formatDateTimeDe(a)} – ${formatDateTimeDe(b)}';
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
