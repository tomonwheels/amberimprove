# amberIMPROVE – Anleitung für amberSUITE

amberIMPROVE ist ein **eigenes Programm** neben amberSUITE: Es schreibt deine
Musikdateien nachts mit **frankls improvefile-Verfahren** bit-identisch neu,
langsam und in genauem Takt. amberSUITE zeigt dir an, was schon improvt ist,
und führt dich zur Status- und Setup-Seite. Umgekehrt achtet amberIMPROVE
auf amberPLAY: Sobald du Musik hörst, pausiert es.

Alles Grundsätzliche (Voraussetzungen, Setup-Felder, was nachts passiert)
steht in der **Anleitung ohne amberSUITE** – hier nur, was für amber-Nutzer
anders oder zusätzlich ist.

---

## 1. Wo installieren?

Auf dem Rechner, **an dem deine Musikplatte hängt** – das ist der Rechner,
dessen Musikordner amberLIBRARY liest (in amberSETUP unter „amberLIBRARY“ zu
sehen, meist der Rechner mit amberPLAY). Installation wie in der
Standalone-Anleitung, Kapitel 2:

```
tar -xzf amberimprove-*-linux-*.tar.gz
cd amberimprove-*/
sudo ./install.sh
```

## 2. Einrichten – die Werte für amberSUITE

Setup-Seite öffnen (`http://<rechner>:8093/#setup`, oder in amberPLAY unter
**Einstellungen → Speicher → amberIMPROVE**, sobald es läuft):

- **Musikordner:** derselbe Ordner, den amberLIBRARY liest (in amberPLAY unter
  Einstellungen → Speicher zu sehen).
- **Wiedergabe-Prüfung:** amberPLAY eintragen:

  ```
  http://127.0.0.1:8082/api/v1/player/playback/state
  ```

  (läuft amberPLAY auf einem anderen Rechner, dessen Adresse statt
  `127.0.0.1`). Feld `engine_state`, Werte `idle, stopped`. Dann pausiert
  amberIMPROVE, sobald amberPLAY spielt, und bricht einen laufenden Titel
  sofort ab – die Datei bleibt dabei unverändert.
- **Zeitfenster:** eine Zeit, in der niemand hört, z. B. 23:30–06:30.
- **Qualität:** frankl „sehr langsam“, 1 Durchgang – wie in der Hauptanlage.

**Speichern.** amberIMPROVE merkt sich dabei die Adresse, unter der du die
Seite geöffnet hast – darüber findet amberPLAY den Weg zurück.

## 3. In amberPLAY sehen

- **Zauberstab im Albumkopf**, neben dem Titel:
  - bernsteinfarben – alle Titel des Albums improvt
  - gedämpft mit Zahl, z. B. „3/12“ – angefangen
  - kein Symbol – noch nicht dran
- **Klick auf den Zauberstab:** je Titel Datum, Verfahren und verpasste Takte
  (0 = jeder Block im Takt), Rohdaten aufklappbar, und
  **„amberIMPROVE öffnen ↗“** zur Statusseite.
- **Einstellungen → Speicher → amberIMPROVE:** der feste Weg zur Status- und
  Setup-Seite – auch bevor das erste Album fertig ist. Ist amberIMPROVE nicht
  installiert, steht dort der Link zum Download.

Die Suite-App (Mac, iPad) zeigt das nach ihrem nächsten Update; im Browser
sofort.

## 4. Zusammenspiel mit Sicherung und Bibliothek

- amberIMPROVE behält **Zeitstempel und Größe** jeder Datei. Eine Sicherung
  per rsync sieht improvte Dateien deshalb als unverändert und kopiert sie
  nicht neu.
- Lege die Sicherung **vor** das Zeitfenster (z. B. 22:30): dann sind neue
  Alben schon gesichert, bevor sie improvt werden.
- amberLIBRARY sieht nur die kleinen `.amberimprove.json`-Dateien; die Titel
  bleiben dieselben, mit denselben Kennungen – Perlen, Tags und Hörverlauf
  bleiben erhalten.
- Bearbeitest du Tags in einer Datei, gilt sie nicht mehr als improvt und
  kommt in einer der nächsten Nächte wieder dran.

## 5. Mehrere Rechner, NAS

- Liegt deine Musik auf einem **NAS** (Synology & Co.) und amberLIBRARY liest
  sie von dort, gehört amberIMPROVE **auf das NAS** – per Container, siehe
  Standalone-Anleitung, Kapitel 7. Über eine Netzfreigabe improven geht
  nicht.
- amberPLAY findet die Statusseite auch dort – über die Adresse, die
  amberIMPROVE beim Speichern im Setup mitbekommen hat.

---

Freie Software (GPL-3.0-or-later): <https://github.com/tomonwheels/amberimprove>.
Verfahren: **frankl** (frankl_stereo); Ablauf mit Prüfung nach jedem Durchgang
nach den **nsc-Skripten von Harald Scherer** (aktives-hoeren.de).
