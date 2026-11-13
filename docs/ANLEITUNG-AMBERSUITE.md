# amberIMPROVE – Anleitung für amberSUITE

amberIMPROVE schreibt deine Musikdateien nachts neu, Titel für Titel, mit
**frankls improvefile-Verfahren**: bit-identisch, aber langsam und in genauem
Takt. Jeder Titel wird danach bitgenau geprüft. Stimmt etwas nicht, bleibt die
Datei, wie sie war. amberPLAY zeigt dir, was schon improvt ist.

## Was du brauchst

- Den **Linux-Rechner, an dem deine Musikplatte hängt** – das ist der
  Rechner, dessen Musikordner amberLIBRARY liest (meist der mit amberPLAY).
- Die Platte muss dort **direkt** hängen (USB, SATA, NVMe), nicht über eine
  Netzfreigabe.
- **Jedes übliche Linux mit systemd:** DietPi, Raspberry Pi OS, Debian,
  Ubuntu, Arch u. a. Das Paket bringt alles mit, nichts muss kompiliert werden.
- Empfehlung: Musikplatte mit **ext4** (auf FAT32/exFAT/NTFS hörte frankl
  schlechtere Ergebnisse).

## Installieren

Passendes Paket laden: **amd64** für PC/NUC, **arm64** für Raspberry Pi.
In amberPLAY findest du den Link unter **Einstellungen → Speicher →
amberIMPROVE**.

```
tar -xzf amberimprove-*-linux-*.tar.gz
cd amberimprove-*/
sudo ./install.sh
```

Am Ende steht die Adresse der Setup-Seite, z. B. `http://192.168.1.50:8093`.
Im Browser öffnen.

## Einrichten

1. **Laufwerk** anklicken, dann mit **Durchsuchen** den Musikordner wählen –
   derselbe Ordner, den amberPLAY unter Einstellungen → Speicher zeigt.
2. **Originale:** „Die Datei selbst“ (Normalfall).
3. **Verfahren:** „Über den Arbeitsspeicher (bufhrt)“ (Voreinstellung) – zum
   zweiten Verfahren siehe unten. **Qualität:** frankl „sehr langsam“ (beste
   Kopie, etwa 3 GB Musik pro Nacht), **1 Durchgang**. Mit **Takt auf diesem Rechner prüfen** testen: 0 verpasste Takte =
   gut.
4. **Zeitfenster:** z. B. 23:30 bis 06:30 – eine Zeit, in der niemand hört.
5. **Wiedergabe-Prüfung:** amberPLAY eintragen:

   ```
   http://127.0.0.1:8082/api/v1/player/playback/state
   ```

   Läuft amberPLAY auf einem anderen Rechner, dessen Adresse statt
   `127.0.0.1`. Sobald amberPLAY spielt, pausiert amberIMPROVE.
6. **Prüfen**, dann **Speichern**. Ab dem nächsten Zeitfenster geht es los.

## Was du in amberPLAY siehst

- **Zauberstab neben dem Albumtitel:** bernsteinfarben = alle Titel improvt,
  gedämpft mit Zahl (z. B. 3/12) = angefangen, kein Zauberstab = noch nicht
  dran.
- **Klick auf den Zauberstab:** je Titel Datum und verpasste Takte
  (0 = perfekt), dazu **„amberIMPROVE öffnen“**.
- **Einstellungen → Speicher → amberIMPROVE:** führt immer zur Status- und
  Setup-Seite.

Perlen, Tags und Hörverlauf bleiben erhalten. Bearbeitest du Tags einer
Datei, kommt sie noch einmal dran.

**Sicherung:** amberIMPROVE behält Zeitstempel und Größe – eine Sicherung per
rsync kopiert improvte Dateien nicht neu. Leg die Sicherung am besten vor das
Zeitfenster (z. B. 22:30).

## NAS (Synology, UGREEN & Co.)

Liegt deine Musik auf einem NAS, gehört amberIMPROVE auf das NAS:

1. Synology: im Paket-Zentrum den **Container Manager** installieren.
   UGREEN (UGOS Pro): im App Center die App **Docker** installieren.
   QNAP, ZimaOS, Unraid: deren Container-Verwaltung.
2. Neues **Projekt** anlegen (UGREEN: in der Docker-App unter „Projekt“) und die mitgelieferte `docker-compose.yml`
   einfügen.
3. Die Zeile `- /volume1/music:/music` auf deinen Musikordner ändern
   (Synology und UGREEN: `/volume1/<Freigabe>/…`).
4. Starten, `http://<nas>:8093` öffnen, als Musikordner **`/music`** wählen,
   weiter wie oben. Bei der Wiedergabe-Prüfung die Adresse deines
   amberPLAY-Rechners eintragen.

## Zwei Verfahren

Unter **Verfahren** wählst du, welches Programm von frankl schreibt:

- **Über den Arbeitsspeicher (bufhrt)** – das bisherige Verfahren und die
  Voreinstellung. Der Titel wird in den Arbeitsspeicher geladen und von dort
  im Takt auf die Platte geschrieben.
- **Direkt (improvedirect)** – neu seit amberIMPROVE 0.2.0, frankls Programm
  `improvedirect` (frankl_stereo ab 0.9.4). Es liest und schreibt abwechselnd
  kleine Blöcke direkt von der Platte und auf die Platte, am Zwischenspeicher
  des Systems vorbei („direct read“, „direct write“). Voreingestellt sind
  frankls eigene Werte: 10 Takte je Sekunde, eine Seite (4096 Bytes) je Takt,
  128 Kopien je Block.

Was bei „Direkt“ anders ist:

- **Es ist deutlich langsamer:** 40 KB je Sekunde statt 184 KB bei „sehr
  langsam“ – rund 26 Sekunden je MB. In eine Nacht von sieben Stunden passen
  rechnerisch etwa 0,8 GB Musik (statt etwa 3 GB), ein einzelner Titel darf
  dann höchstens etwa 850 MB groß sein. Größere Titel meldet das Protokoll
  und lässt sie aus.
- Alles andere bleibt gleich: Zeitfenster, Wiedergabe-Prüfung, bitgenaue
  Prüfung nach jedem Durchgang. Endet das Zeitfenster mitten in einem Titel,
  wird er abgebrochen, die Datei bleibt, wie sie war, und in der nächsten
  Nacht kommt er von vorn dran.
- Die Platte muss direktes Lesen und Schreiben erlauben (ext4 und die anderen
  üblichen Linux-Dateisysteme tun das). **Prüfen** sagt es dir.
- **Wechselst du das Verfahren, werden schon improvte Titel noch einmal
  geschrieben** – sie zählen dann als „älteres Verfahren“.

Welches besser klingt, entscheidet dein Ohr.

## Aktualisieren und entfernen

- Update: neues Paket entpacken, `sudo ./install.sh`. Einstellungen bleiben.
- Entfernen: `sudo ./uninstall.sh`. Deine Musik bleibt, wie sie ist.

---

Freie Software (GPL-3.0-or-later), <https://github.com/tomonwheels/amberimprove>.
Verfahren und die Programme `bufhrt` und `improvedirect` von **frankl**
(frankl's stereo utilities, GPL-3.0-or-later,
<https://github.com/frankl-audio/frankl_stereo>; unverändert mitgeliefert:
bufhrt aus v0.9.3, improvedirect aus v0.9.4), Ablauf nach den Skripten von
**Harald Scherer** (aktives-hoeren.de).
