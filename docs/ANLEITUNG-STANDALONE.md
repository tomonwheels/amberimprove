# amberIMPROVE – Anleitung (ohne amberSUITE)

amberIMPROVE schreibt deine Musikdateien nachts, Titel für Titel, mit
**frankls improvefile-Verfahren** neu: bit-identisch – kein einziges Bit der
Musik ändert sich –, aber sehr langsam und in genauem Takt auf die Platte,
von der du abspielst. Wer das Verfahren aus dem Forum aktives-hoeren.de
kennt: amberIMPROVE macht genau das, nur automatisch, jede Nacht ein Stück,
mit Prüfung nach jedem Schritt.

---

## 1. Was du brauchst

- **Einen Linux-Rechner, an dem die Musikplatte direkt hängt** – per USB,
  SATA oder NVMe. Das kann ein Raspberry Pi 4/5 sein, ein NUC, ein alter PC
  oder ein NAS (siehe Kapitel 7).
- **Nicht über das Netz:** Liegt die Musik auf einem NAS und ist am
  Linux-Rechner nur als Freigabe (SMB/NFS) eingebunden, landet der genaue
  Takt nicht auf der Platte, sondern verpufft im Netzwerk. Das Setup lehnt
  solche Ordner ab. amberIMPROVE muss auf dem Gerät laufen, **an dem die Platte
  steckt**.
- **macOS und Windows gehen nicht** – frankls Programm `bufhrt` ist ein reines
  Linux-Programm.
- Eine Sicherung deiner Musik schadet nie. amberIMPROVE prüft zwar jeden
  Titel bitgenau, bevor es ihn ersetzt – aber eine Sicherung gehört zu jeder
  Musiksammlung.

**Tipp zum Dateisystem:** frankl hörte auf **ext4** bessere Ergebnisse als auf
FAT32, exFAT oder NTFS. amberIMPROVE funktioniert auf allen, warnt aber im
Setup.

## 2. Installieren

1. Von der Download-Seite das passende Paket holen:
   - `amberimprove-…-linux-amd64.tar.gz` – normale PCs, NUCs, Intel/AMD
   - `amberimprove-…-linux-arm64.tar.gz` – Raspberry Pi 4/5 und andere ARM64
2. Auf den Linux-Rechner kopieren, entpacken und installieren:

   ```
   tar -xzf amberimprove-*-linux-*.tar.gz
   cd amberimprove-*/
   sudo ./install.sh
   ```

3. Am Ende steht die Adresse der Setup-Seite, z. B.
   `http://192.168.1.50:8093/#setup`. Die öffnest du auf einem beliebigen
   Rechner, Tablet oder Handy im selben Netz.

## 3. Einrichten (Setup-Seite)

Die Seite gibt es auf Deutsch und Englisch (oben rechts umschalten).

1. **Laufwerk mit der Musik** – das Laufwerk anklicken, von dem du abspielst.
   Darunter mit **Durchsuchen** den Ordner mit der Musik wählen
   („Diesen Ordner wählen“). Den Arbeitsordner legt amberIMPROVE selbst an,
   auf derselben Platte.
2. **Woher kommen die Originale?**
   - **Die Datei selbst** – der Normalfall. Jede Datei wird in den
     Arbeitsspeicher gelesen, neu geschrieben und bitgenau verglichen.
   - **Ein Ordner auf diesem Rechner** – wenn du eine zweite Kopie auf einer
     anderen Platte oder eingebundenen Freigabe hast.
   - **Ein anderer Rechner (rsync über ssh)** – wenn die Sicherung auf einem
     Server liegt (für Fortgeschrittene; der Schlüssel sollte dort nur lesen
     dürfen).
3. **Qualität**
   - **frankl „sehr langsam“** – die beste Kopie laut den Forumstests.
     Rechne grob mit **3 GB Musik pro Nacht** (7 Stunden).
   - **Langsam** und **Zügig** – schneller fertig.
   - **Durchgänge je Titel** – 1 reicht; mehr Durchgänge dauern entsprechend
     länger.
   - **Takt auf diesem Rechner prüfen** – schreibt 1 MB zur Probe.
     **0 verpasste Takte** heißt: dein Rechner hält frankls Takt.
4. **Zeitfenster** – z. B. 23:30 bis 06:30. Nur in dieser Zeit wird
   gearbeitet. Ein Titel wird nur begonnen, wenn er vor dem Ende fertig wird.
5. **Wiedergabe-Prüfung** – hat dein Player eine Statusadresse (JSON), trag
   sie ein; dann pausiert amberIMPROVE, sobald Musik läuft. Ohne Eintrag
   schützt nur das Zeitfenster – dann sollte in dieser Zeit niemand hören.
6. **Prüfen**, dann **Speichern**. Ab dem nächsten Zeitfenster geht es los.
   Das Setup sagt auch, wie viele Nächte es für deine ganze Sammlung braucht.

## 4. Was nachts passiert

Für jeden Titel:

1. Original in den Arbeitsspeicher holen und mit der Datei auf der Platte
   vergleichen – weicht etwas ab, wird der Titel **übersprungen, nie
   überschrieben**.
2. Mit frankls `bufhrt` langsam und im Takt auf die Platte schreiben.
3. Zurücklesen und **bitgenau vergleichen**. Stimmt etwas nicht: bis zu zehn
   neue Versuche, sonst bleibt die Datei, wie sie war.
4. Erst dann ersetzt die neue Datei die alte – mit gleichem Zeitstempel,
   gleichen Rechten.
5. Eintrag in `.amberimprove.json` im Albumordner.

Beginnt mitten in einem Titel die Wiedergabe (oder endet das Zeitfenster),
wird sofort abgebrochen – die Datei bleibt unverändert.

## 5. Woran du siehst, was improvt ist

- **Übersicht** auf der Statusseite: Fortschritt, laufender Titel, Protokoll.
- **Alben**: Zauberstab in Bernstein = fertig, gedämpft mit Zahl = angefangen.
  Klick zeigt je Titel Datum, Verfahren und **verpasste Takte** (0 = perfekt).
- Im Albumordner liegt `.amberimprove.json` – eine kleine Textdatei, die
  Player auslesen können.

Ändert sich eine Datei später (z. B. neue Tags), gilt sie nicht mehr als
improvt und wird in einer der nächsten Nächte erneut behandelt. Wechselst du
die Qualitätsstufe, werden alle Titel mit der neuen Stufe noch einmal
behandelt.

## 6. Aktualisieren und Entfernen

- **Neue Version:** neues Paket entpacken, `sudo ./install.sh` – Einstellungen
  und Protokolle bleiben.
- **Entfernen:** `sudo ./uninstall.sh` (Einstellungen bleiben) oder
  `sudo ./uninstall.sh --alles`. Deine Musik bleibt in jedem Fall, wie sie ist.

## 7. Synology und andere NAS (Container)

Auf einem NAS installiert man keine Programme von Hand – dafür gibt es
amberIMPROVE als **Container**.

**Synology (DSM 7, Container Manager):**

1. Im Paket-Zentrum den **Container Manager** installieren. (Er läuft auf den
   meisten „+“-Modellen mit Intel/AMD-Prozessor; ältere 32-Bit-ARM-Modelle
   gehen nicht.)
2. In der File Station einen Ordner anlegen, z. B. `/docker/amberimprove`.
3. Container Manager → **Projekt** → **Erstellen** → Pfad
   `/docker/amberimprove` → `docker-compose.yml` aus dem Download einfügen.
4. In der Datei die Zeile `- /volume1/music:/music` auf deinen Musikordner
   ändern (z. B. `/volume1/Musik:/music`).
5. Projekt starten, dann im Browser `http://<nas-adresse>:8093/#setup`
   öffnen. Als Musikordner **`/music`** wählen, weiter wie in Kapitel 3.

**QNAP, ZimaOS, Unraid, TrueNAS:** genauso über deren Container-Oberfläche mit
derselben `docker-compose.yml`.

**Gut zu wissen:**

- Die Platten eines NAS stecken meist in einem RAID-Verbund. Ob frankls
  Verfahren dort genauso wirkt wie auf einer einzelnen SSD, weiß niemand –
  das musst du hören.
- `shm_size: "2gb"` in der Datei nicht entfernen: dort liegen die Originale
  während der Arbeit im Arbeitsspeicher.

## 8. Hilfe

- Statusseite → **Protokoll**: jeder Schritt mit Uhrzeit, Fehler in Rot.
- Auf dem Linux-Rechner: `journalctl -u amberimprove` bzw. beim Container
  „Protokoll“ im Container Manager.
- Quelltext und Fragen: <https://github.com/tomonwheels/amberimprove>

amberIMPROVE ist freie Software (GPL-3.0-or-later). Das Verfahren stammt von
**frankl** (frankl_stereo), der Ablauf mit Prüfung nach jedem Durchgang folgt
den **nsc-Skripten von Harald Scherer** – beide aus dem Forum
aktives-hoeren.de.
