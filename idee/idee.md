# Desksharing

Plattform zur Verwaltung und Reservierung von Desksharing-Plätzen. Platztypen und Ausstattungen sind definierbar; Ausstattungen dienen als Add-ons an Plätzen, damit nicht für jede Kombination ein eigener Platztyp nötig ist. Zusätzlich gibt es eigenständige, ortsgebundene Ressourcen (z. B. Beamer).

**Betrieb:** On-Prem (eigene Firma) oder bei einem Hoster — dieselbe Software, dieselbe API.

**Clients:** Verwaltung (Standorte, Gebäude, Plätze, Benutzer, Ausstattung, …) ausschließlich am **Computer / Web**. Die **Mobile-App** kommt später und dient nur dem Buchen (Suche, Buchung, Status, Check-in/out, Erinnerungen) — keine Verwaltungsfunktionen.

# Platformadministrator

Verwaltet die Mandanten (in der Regel Firmen).

# Mandanten

Ein Mandant kann sich selbst registrieren und erhält eine E-Mail mit Bestätigungslink. Pro Mandant gibt es einen oder mehrere Mandanten-Admins.

# Rollen und Rechte

- **Platform-Admin:** Mandanten verwalten
- **Mandanten-Admin:** Gebäude, Plätze, Ressourcen, Benutzer; kann Sperr-Recht entziehen
- **Mitarbeiter:** buchen, check-in/out, Plätze/Räume sperren

Sperren darf jeder. Bei Missbrauch entzieht der Admin dem Benutzer das Sperr-Recht.

# Authentifizierung

- **Phase 1:** E-Mail / Passwort (inkl. Bestätigungslink bei Registrierung)
- **Später:** LDAP / Active Directory (die meisten Firmen haben das) — pro Mandant zuschaltbar, parallel zu lokalem Login
- **Architektur von Anfang an:** Benutzer über eine Identity-Provider-Abstraktion (lokal vs. LDAP), damit LDAP später ohne Umbau des User-Modells angebunden werden kann. LDAP selbst ist kein MVP.

# Standorte

Pro Standort eine eigene Übersichtskarte (Zoom und Pan reichen). Hat ein Mandant einen großen Standort, kann ein Plan hochgeladen werden; Gebäude lassen sich als Polygon erfassen — auch wenn es keine öffentliche Adresse gibt. Mehrere Standorte in einer Stadt = mehrere Karten.

## Gebäudedefinitionen

Der Mandanten-Admin verwaltet Gebäude (Adresse / GPS-Koordinaten), Stockwerke und Räume. Jedes Gebäude hat ein Kürzel. Platznummer:

`Gebäudekürzel-Stockwerk-Raumnummer-Platznummer`

**Grafischer Editor (nur Web):** Stockwerkpläne hochladen; **Räume und Plätze zunächst als Rechtecke** auf dem Plan (später optional Polygone für unregelmäßig geformte Räume). Die Mobile-App hat keinen Editor.

## Platzdefinitionen

- **Platztypen** (z. B. Arbeitsplatz 2 Bildschirme, Arbeitsplatz 3 Bildschirme)
- **Ausstattungen als Add-ons** am Platz (z. B. 2 Dockingstations, KI-Grafikkarte, geheimes Netzwerk)
- Buchungsmodus pro Platz: **tageweise** (Default) oder **stundenweise**

## Ressourcen

Eigenständige Objekte, **ortsgebunden** (Standort, ggf. Gebäude/Raum). Unabhängig von Plätzen buchbar; im Buchungsflow optional zusammen mit einem Platz am gleichen Standort.

## Buchung

- Für **Mitarbeiter** und **Teams**
- Belegte Plätze sind nicht buchbar (keine Überbuchung)
- Freie Plätze: grün; belegte: rot; Filter z. B. „3 Bildschirme + 2 Dockingstations“

**Ziel-Flow (Web und später Mobile):**  
Standort (und ggf. Gebäude) wählen → Stockwerk wählen → **Gebäude-/Stockwerkplan** öffnen → freien Platz auf dem Plan antippen/klicken und buchen. Filter (Platztyp, Ausstattung, Zeitraum) schränken ein, welche Plätze auf dem Plan als buchbar hervorgehoben werden.

**Übergang (jetzt):** Listen-/Wochenraster-Suche und Raum-Wochenübersicht, bis Stockwerkpläne und Platz-Geometrie im Editor gepflegt sind. Die kaskadierende Auswahl Standort → Gebäude → Stockwerk → Raum bereitet denselben Navigationseinstieg vor.

### Sperren

Zwei Ebenen (einfacher bei asymmetrischen Büros als reine „Zonen“):

1. **n Plätze sperren** (z. B. Team für eine Woche)
2. **Ganzes Büro/Raum nicht buchbar** machen (Wartung, Umbau, Projektfläche)

### Check-in / Check-out

- Check-in ab **15 Minuten vor** Buchungsbeginn
- Bei **stundenweiser** Buchung zusätzlich Check-out
- **No-Show:** 15 Minuten nach Terminbeginn ohne Check-in → Platz gilt als **möglicherweise frei** (Freigabe / erneute Buchbarkeit)

### Erinnerungen (Mobile-App)

Erinnerungen in der Handy-App (z. B. Check-in offen) — lokal und ggf. optional per Push.

# Mobile-App

Android-App (Flutter), Download über die Website. Dieselbe API wie die Web-App, aber **nur Buchungs-Workflow**: Suche, Buchen, Status, Check-in/out, Erinnerungen. **Keine** Verwaltung von Standorten, Gebäuden, Stockwerken, Plätzen, Benutzern oder Katalogen — das bleibt Web/Desktop.

# Datenschutz

Für Statistik nur: ein Tag in der Vergangenheit war belegt — **nicht**, von wem. Nach **1 Jahr** werden diese Daten nicht mehr gespeichert.

# Backend

Server in **Go**; alle nötigen Dateien werden eingebettet. Eine API für Web und Flutter.

# Installation

- Installationsskript (u. a. Datenbank anlegen)
- Linux: `.deb` zur Installation
- Updateskript für das Programm
- Skript für Rechte/Besitzer nach Aktualisierung der Mobile-App-Artefakte
