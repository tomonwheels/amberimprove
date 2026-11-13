# Änderungen

## 0.2.0 (in Vorbereitung)

**Neu: zweites Verfahren „Direkt“ mit frankls `improvedirect`.**

frankl hat seinen stereo utilities mit Version 0.9.4 ein neues Programm für
improvefile mitgegeben: `improvedirect`. Es lädt die Musik nicht erst in den
Arbeitsspeicher, sondern liest und schreibt abwechselnd kleine Blöcke direkt
von der Platte und auf die Platte, am Zwischenspeicher des Systems vorbei
(„direct read“ und „direct write“). amberIMPROVE kann die Durchgänge jetzt
wahlweise damit schreiben.

- **Wahl des Verfahrens** auf der Setup-Seite (Karte „Verfahren“) und in der
  Konfiguration (`"procedure": "bufhrt"` oder `"direct"`, Parameter im Block
  `"direct"`). **Voreinstellung bleibt das bisherige Verfahren (bufhrt)** –
  wer nichts ändert, bekommt nach dem Update genau dasselbe wie vorher, und
  schon improvte Titel bleiben „fertig“.
- Voreingestellt für „Direkt“ sind frankls eigene Werte aus seinem
  improvefile-Skript: `improvedirect -v -o -i -w … 10 40960 10 128`
  (10 Takte je Sekunde, eine Seite von 4096 Bytes je Takt, Sync alle
  10 Takte, 128 Kopien je Block, direktes Lesen und Schreiben).
- Alles drumherum ist für beide Verfahren gleich: Zeitfenster und
  Wiedergabe-Prüfung, bitgenaue Prüfung nach jedem Durchgang, Wiederholungen,
  Ersetzen erst ganz am Ende. Ein abgebrochener Titel bleibt unberührt und
  kommt in der nächsten Nacht von vorn dran.
- **„Direkt“ ist deutlich langsamer:** 40 KB je Sekunde, rund 26 Sekunden je
  MB (bufhrt „sehr langsam“: rund 6 Sekunden je MB). Die Zeitschätzung
  (Nächte, „passt der Titel noch ins Fenster?“) rechnet mit dem gewählten
  Verfahren.
- Der Albumeintrag (`.amberimprove.json`) nennt das Verfahren
  (`"procedure"`) und dessen Parameter; als Verfahrensname steht
  `frankl-0.9.4/direkt/…`. Wer das Verfahren wechselt, lässt schon improvte
  Titel noch einmal schreiben („älteres Verfahren“).
- „Prüfen“ auf der Setup-Seite sieht bei „Direkt“ nach, ob `improvedirect`
  da ist und ob die Platte direktes Lesen und Schreiben erlaubt. Die
  Taktprobe läuft mit dem gewählten Verfahren.
- Pakete, Container und `scripts/build-frankl.sh` bringen `improvedirect`
  zusätzlich zu `bufhrt` mit.

Herkunft: `improvedirect` aus frankl's stereo utilities, Tag v0.9.4 (Commit
`df48764f32fd48b2576afbefe40b13d1e4f498e9`), unverändert, Copyright frankl,
GPL-3.0-or-later, <https://github.com/frankl-audio/frankl_stereo>. `bufhrt`
bleibt bei v0.9.3 (Commit `f41973039f49ae6b6e3f729691c4cfce2bad9da0`).

## 0.1.1

- Favicon.
- Ein Hinweis statt eines Absturzes, wenn die Einstellungen nicht laden.

## 0.1.0

- Erste Fassung: frankls improvefile nachts, Titel für Titel.
