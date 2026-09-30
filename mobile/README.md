# Desksharing

Android-App zum Buchen und Stornieren von Plätzen. Dieselbe API wie die Web-Oberfläche, ohne Verwaltung.

## Start

Der Go-Server muss laufen (`HTTP_ADDR`, lokal `:8081`).

```bash
cd mobile
flutter pub get
flutter run
```

Serveradresse auf dem Anmeldebildschirm:

- Android-Emulator: `http://10.0.2.2:8081`
- Gerät im selben WLAN: `http://<IP-des-Rechners>:8081`

## Umfang

Anmeldung, Auswahl Standort → Gebäude → Stockwerk → Raum, Woche antippen und buchen (ganzer Tag 08:00–18:00 oder freier Zeitraum), eigene Buchungen stornieren.
