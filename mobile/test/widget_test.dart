import 'package:desksharing/main.dart';
import 'package:desksharing/session.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('login screen offers sign-in', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(DesksharingApp(session: Session(prefs)));
    await tester.pumpAndSettle();

    expect(find.text('Desksharing'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Anmelden'), findsOneWidget);
    expect(find.text('E-Mail'), findsOneWidget);
    expect(find.text('Passwort'), findsOneWidget);

    await tester.tap(find.widgetWithText(FilledButton, 'Anmelden'));
    await tester.pump();
    expect(find.text('E-Mail und Passwort sind erforderlich.'), findsOneWidget);
  });
}
