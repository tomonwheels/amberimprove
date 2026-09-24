# amberIMPROVE — Plan

Stand 23.09.2026. Eigenständiges GPL-Programm, in amberSUITE nur verlinkt
(Muster: amberFOCUS Setup). Die Suite liest ausschließlich die Protokolldatei
`.amberimprove.json` (Daten), nie Code.

## Ziel

Die Musikdateien auf der Musik-SSD am NUC werden nachts, Titel für Titel, mit
frankls `improvefile`-Verfahren (bufhrt `--interval`) neu geschrieben:
bit-identisch, aber langsam und in genauem Takt. Improvte Titel und Alben sind
in amberPLAY an einem Zauberstab im Albumkopf erkennbar.

## Entscheidungen (Tom, 23.09.)

- Name **amberIMPROVE**, Repo öffentlich unter `tomonwheels`, GPL-3.0-or-later.
- Musik-SSD am NUC wird **ext4** (Umzug über den Zima, siehe unten).
- **Originale liegen auf dem Zima** (`sdb2/Backup/Musik`, nächtliche Sicherung
  NUC → Zima). Die improvte Kopie wird aus dem Zima-Original geschrieben und
  ersetzt die Datei auf der SSD. Die Änderungszeit bleibt erhalten, damit die
  Sicherung die Datei überspringt und auf dem Zima das Original bleibt.
- Kennzeichnung: `.amberimprove.json` je Albumordner. Zauberstab oben im
  Album-View, Klick öffnet Popup mit dem Inhalt.
- Qualität soll auch vom minimalen Linux abhängen → Hörvergleich
  NUC-Nachtlauf gegen Haralds RamRoot-Stick.
- Qualität: **frankl „sehr langsam“, 1 Durchgang** (Tom). Auf dem NUC
  gemessen: 0 verpasste Takte; Bibliothek (2563 Titel, 112 GB) ≈ 34 Nächte.
- Sicherung NUC → Zima auf **22:30** verlegen (vor dem Improve-Fenster
  23:30–06:30), keine Sperrzeiten.
- Universell: Setup-Seite, keine festen Rechner im Code.

## Ablauf je Titel

1. **Wächter:** nur im Nachtfenster (optionale Sperrzeiten, bei Tom keine),
   nur wenn amberPLAY nichts spielt. Beginnt eine Wiedergabe
   mitten im Titel, wird sofort abgebrochen; die Datei auf der SSD bleibt
   unberührt (ersetzt wird erst ganz am Ende per `rename`).
2. **Original holen:** vom Zima per rsync in eine RAM-Disk (`/dev/shm`).
3. **Gegenprobe:** Größe und Änderungszeit wie auf der SSD (±2 s) und
   SHA-256 des Originals = SHA-256 der SSD-Datei. Weicht etwas ab (neues Album
   noch nicht gesichert, Datei geändert), wird der Titel übersprungen — nie
   überschrieben.
4. **Durchgänge** (einstellbar 1–8, bei Tom 1; Haralds nsc nimmt 3): RAM →
   Arbeitsdatei auf der SSD → ggf. weitere Arbeitsdateien → letzte Arbeitsdatei. Jeder
   Durchgang = `bufhrt --interval` mit frankls Parametern. Nach jedem
   Durchgang: fsync, Datei aus dem Seitencache werfen (`FADV_DONTNEED`, statt
   Haralds Aus-/Einhängen, weil amberPLAY die SSD liest), vom Datenträger
   zurücklesen, SHA-256 vergleichen. Bei Abweichung bis zu 10 Wiederholungen,
   dann Abbruch des Titels.
5. **Ersetzen:** `rename` der letzten Arbeitsdatei auf den Originalnamen
   (gleiches Dateisystem, nur Metadaten), Änderungszeit, Besitzer und Rechte
   des Originals setzen, erneut aus dem Cache werfen.
6. **Protokoll:** Eintrag in `.amberimprove.json` des Albumordners (atomar
   geschrieben).

Arbeitsordner liegt auf derselben Partition, aber **außerhalb** von
`/mnt/Music/Music` (`/mnt/Music/.amberimprove-tmp`) — so sieht ihn weder die
Bibliothek noch die nächtliche Sicherung.

## Protokolldatei `.amberimprove.json`

```json
{
  "tool": "amberIMPROVE",
  "format": 1,
  "files": {
    "01 Titel.flac": {
      "size": 31457280,
      "mtime": "2024-05-02T10:11:12Z",
      "sha256": "…",
      "improved_at": "2026-09-24T01:12:00+02:00",
      "method": "frankl-0.9.3/sehr-langsam/1x",
      "passes": 1,
      "host": "amber-nuc",
      "params": { "bytes_per_second": 188044, "…": "…" },
      "delayed_blocks": 0
    }
  }
}
```

Ein Titel gilt in der Suite als improvt, solange Größe und Änderungszeit zur
Datei passen. Ändert sich der Verfahrensstand (`method`), sind die Titel
„improvt (älteres Verfahren)“ und kommen wieder in die Warteschlange.

## Bausteine

- `amberimprove run` — Dauerdienst: wartet auf das Zeitfenster, arbeitet die
  Nacht ab, liest die Konfiguration vor jeder Nacht neu. Titel werden nur
  begonnen, wenn sie vor Fensterende fertig werden; Titel, die in keine Nacht
  passen, werden gemeldet und übersprungen.
- `amberimprove plan` — zeigt die Warteschlange, ändert nichts.
- `amberimprove once <Pfad>` — genau einen Titel improven (Test).
- `amberimprove status` — Zustand im Terminal.
- `amberimprove serve` — Status- und Setup-Seite (DE/EN): Laufwerk,
  Musikordner, Quelle der Originale (Datei selbst / Ordner / rsync),
  Qualitätsstufe (frankl „sehr langsam“, langsam, zügig), Durchgänge,
  Zeitfenster, Wiedergabe-Prüfung, Taktprüfung, Prüfen + Speichern.
- Konfiguration `/etc/amberimprove/config.json`.
- bufhrt: eigener Bau aus frankl_stereo v0.9.3 (`scripts/build-frankl.sh`),
  nicht der für die Suite angepasste bufhrt.

## Umzug der SSD auf ext4 (läuft)

1. Prüfsummen am NUC + Kopie NUC → `sdb2/Umzug-Musik-2026-09` ✅ gestartet
2. amber-library, amber-play, Roon stoppen; `sda2` → ext4 (Label MUSIC),
   fstab neue UUID, `noatime` — nur auf Toms ausdrückliches Go
3. Zurückspielen mit gleichen Pfaden und Zeiten, `chown amber:amber`,
   Prüfsummen vergleichen
4. Dienste starten (library, play, roon), prüfen
5. Umzugsordner löschen, wenn Tom gehört hat

## Suite-Seite (danach, proprietär)

- amber-library liest `.amberimprove.json` beim Durchsuchen, liefert je
  Album `improved: n/m` und je Titel den Status.
- amberPLAY: Zauberstab im Albumkopf (bernstein = ganz, gedämpft + „3/12“ =
  teilweise), Klick → bestehender Dialog mit Inhalt, Rohdaten aufklappbar.
  Übersetzungsschlüssel, keine harten Texte.
- Link auf amberIMPROVE (Releases) in amberPLAY.

## Stand 23.09. abends

Programm gebaut (`~/Dev/amberimprove`, lokal, noch nicht veröffentlicht),
Unit-Tests grün, auf dem NUC im Probeordner getestet: bitgleich, Zeitstempel
erhalten, Datei per rename ersetzt, Abbruch lässt Datei unverändert,
abweichendes Original wird übersprungen, Setup-Schnittstellen (Laufwerke,
Prüfen, Taktprüfung, Speichern, Schutz gegen Programmpfad aus dem Browser).

## Offen

- Lesezugang NUC → Zima (eigener Schlüssel, nur lesend auf `Backup/Musik`).
- Kern für bufhrt (NUC: `isolcpus=4,5` für die Wiedergabe; nachts frei?) und
  feste CPU-Frequenz während des Laufs.
- Roon-Wiedergabe wird vom Wächter nicht erkannt (nur amberPLAY).
- Hörvergleich gegen Haralds Stick.
