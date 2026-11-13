# amberIMPROVE – Anleitung

amberIMPROVE schreibt deine Musikdateien nachts neu, Titel für Titel, mit
**frankls improvefile-Verfahren**: bit-identisch, aber langsam und in genauem
Takt. Jeder Titel wird danach bitgenau geprüft. Stimmt etwas nicht, bleibt die
Datei, wie sie war.

## Was du brauchst

- Einen **Linux-Rechner, an dem die Musikplatte direkt hängt** (USB, SATA,
  NVMe) – z. B. Raspberry Pi 4/5, NUC, PC oder ein NAS (siehe unten).
- Nicht über das Netz: Eine Netzfreigabe (SMB/NFS) geht nicht – der Takt
  käme nicht auf der Platte an.
- macOS und Windows gehen nicht.
- **Jedes übliche Linux mit systemd:** DietPi, Raspberry Pi OS, Debian,
  Ubuntu, Arch u. a. Das Paket bringt alles mit, nichts muss kompiliert werden.
- Empfehlung: Musikplatte mit **ext4** (auf FAT32/exFAT/NTFS hörte frankl
  schlechtere Ergebnisse).

## Installieren

Passendes Paket laden: **amd64** für PC/NUC, **arm64** für Raspberry Pi.

```
tar -xzf amberimprove-*-linux-*.tar.gz
cd amberimprove-*/
sudo ./install.sh
```

Am Ende steht die Adresse der Setup-Seite, z. B. `http://192.168.1.50:8093`.
Im Browser öffnen – auf jedem Gerät im Heimnetz.

## Einrichten

1. **Laufwerk** anklicken, dann mit **Durchsuchen** den Musikordner wählen.
2. **Originale:** „Die Datei selbst“ (Normalfall).
3. **Verfahren:** „Über den Arbeitsspeicher (bufhrt)“ (Voreinstellung) – zum
   zweiten Verfahren siehe unten. **Qualität:** frankl „sehr langsam“ (beste
   Kopie, etwa 3 GB Musik pro Nacht), **1 Durchgang**. Mit **Takt auf diesem Rechner prüfen** testen: 0 verpasste Takte =
   gut.
4. **Zeitfenster:** z. B. 23:30 bis 06:30 – eine Zeit, in der niemand hört.
5. **Wiedergabe-Prüfung:** leer lassen, oder die Statusadresse deines Players
   eintragen, falls er eine hat. Dann pausiert amberIMPROVE, sobald Musik
   läuft.
6. **Prüfen**, dann **Speichern**. Ab dem nächsten Zeitfenster geht es los.

## Was du siehst

Die Statusseite (`http://<rechner>:8093`) zeigt Fortschritt, laufenden Titel
und Protokoll. Bei den Alben: **Zauberstab in Bernstein** = fertig,
**gedämpft mit Zahl** (z. B. 3/12) = angefangen. Ein Klick zeigt je Titel
Datum und verpasste Takte (0 = perfekt).

Wird eine Datei später verändert (z. B. neue Tags), kommt sie noch einmal dran.

## NAS (Synology, UGREEN & Co.)

1. Synology: im Paket-Zentrum den **Container Manager** installieren.
   UGREEN (UGOS Pro): im App Center die App **Docker** installieren.
   QNAP, ZimaOS, Unraid: deren Container-Verwaltung.
2. Neues **Projekt** anlegen (UGREEN: in der Docker-App unter „Projekt“) und die mitgelieferte `docker-compose.yml`
   einfügen.
3. Die Zeile `- /volume1/music:/music` auf deinen Musikordner ändern
   (Synology und UGREEN: `/volume1/<Freigabe>/…`).
4. Starten, `http://<nas>:8093` öffnen, als Musikordner **`/music`** wählen,
   weiter wie oben.

Hinweis: NAS-Platten laufen meist im RAID. Ob das Verfahren dort genauso
wirkt, musst du hören.

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
