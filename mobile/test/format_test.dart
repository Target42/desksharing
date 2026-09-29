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

  test('single option is selected when nothing was saved', () {
    expect(preferId(null, ['only']), 'only');
    expect(preferId('kept', ['kept', 'other']), 'kept');
    expect(preferId('missing', ['a', 'b']), isNull);
  });
}
