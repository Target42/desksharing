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

## Download über die Website

Release-APK bauen und ins Datenverzeichnis des Servers legen. Der Dateiname muss `desksharing.apk` sein. Liegt genau eine andere `.apk` in dem Ordner, wird die ebenfalls ausgeliefert.

```bash
flutter build apk --release
sudo install -m 640 -o desksharing -g desksharing \
  build/app/outputs/flutter-apk/app-release.apk \
  /var/lib/desksharing/mobile/desksharing.apk
sudo desksharing-fix-mobile-perms
```

Lokal heißt der Ordner `data/mobile` (`DATA_DIR`). Die Anmeldung zeigt dann „Android-App laden“. Die Datei liegt unter `/download/desksharing.apk`. Ein Neustart des Servers ist dafür nicht nötig.

## Umfang

Anmeldung, Auswahl Standort → Gebäude → Stockwerk → Raum, Woche antippen und buchen (ganzer Tag 08:00–18:00 oder freier Zeitraum), eigene Buchungen stornieren.
