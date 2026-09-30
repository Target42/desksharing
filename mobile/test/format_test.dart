import 'package:desksharing/api.dart';
import 'package:desksharing/format.dart';
import 'package:desksharing/session.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('monday of 29 Sep 2026 is 28 Sep', () {
    expect(ymd(mondayOf(DateTime(2026, 9, 29))), '2026-09-28');
  });

  test('week span uses inclusive end date', () {
    expect(formatWeekSpan('2026-09-28', '2026-10-04'), '28. Sep. – 04. Okt.');
  });

  test('normalizeBaseUrl adds scheme and strips slash', () {
    expect(normalizeBaseUrl('127.0.0.1:8081/'), 'http://127.0.0.1:8081');
    expect(normalizeBaseUrl(''), 'http://127.0.0.1:8081');
  });

  test('apiErrorMessage prefers server message', () {
    expect(
      apiErrorMessage(
        '{"error":"conflict","message":"place already booked"}',
        409,
      ),
      'place already booked',
    );
    expect(apiErrorMessage('nope', 500), 'Anfrage fehlgeschlagen (500).');
  });

  test('whole-day booking shows inclusive dates', () {
    expect(
      formatBookingRange(DateTime(2026, 10, 1), DateTime(2026, 10, 4)),
      '01.10.2026 – 03.10.2026',
    );
  });

  test('evening resource slot moves to next morning', () {
    final slot = defaultResourceHourly(DateTime(2026, 9, 30, 21, 50));
    expect(ymd(slot.day), '2026-10-01');
    expect(slot.start, '09:00');
    expect(slot.end, '12:00');
  });

  test('afternoon resource slot starts at the next quarter', () {
    final slot = defaultResourceHourly(DateTime(2026, 9, 30, 15, 7));
    expect(ymd(slot.day), '2026-09-30');
    expect(slot.start, '15:15');
    expect(slot.end, '17:15');
  });

  test('single option is selected when nothing was saved', () {
    expect(preferId(null, ['only']), 'only');
    expect(preferId('kept', ['kept', 'other']), 'kept');
    expect(preferId('missing', ['a', 'b']), isNull);
  });
}
