# amberIMPROVE

Rewrites the music files on your playback medium with **frankl's improvefile
procedure** — title by title, at night, bit-identical, verified after every
pass — and records every improved title in a small file per album
(`.amberimprove.json`). Players such as amberPLAY can read that file and mark
improved albums.

## How it works

For every title:

1. **Guards:** only inside the nightly time window, and only while nothing is
   playing (asks the player's HTTP status). If playback starts in the middle of
   a title, the title is abandoned at once — the file on the medium stays
   exactly as it was.
2. **Original:** the untouched original is fetched from a second copy (e.g. a
   backup server via rsync, or a local path) into RAM.
3. **Cross-check:** size, modification time and SHA-256 of the original must
   match the file on the medium. Otherwise the title is skipped, never
   overwritten.
4. **Passes:** `bufhrt --interval` writes the file slowly and in precise timing
   onto the medium, several times (default 3). After each pass the file is
   flushed, dropped from the page cache, read back from the medium and compared
   (SHA-256). Up to 10 retries per pass. bufhrt's `delayed block` messages are
   counted and recorded: they show how evenly the pass was written.
5. **Replace:** the last pass file is renamed onto the original name (same
   filesystem, metadata only) with the original's time, owner and mode.
6. **Record:** the entry goes into the album's `.amberimprove.json`.

## Guides (German)

- [docs/ANLEITUNG-STANDALONE.md](docs/ANLEITUNG-STANDALONE.md) — install and
  set up on any Linux machine, plus Synology/NAS via container
- [docs/ANLEITUNG-AMBERSUITE.md](docs/ANLEITUNG-AMBERSUITE.md) — together with
  amberSUITE (wand in amberPLAY, playback check)

## Install

Download the package for your machine from the releases page, then:

```
tar -xzf amberimprove-*-linux-*.tar.gz && cd amberimprove-*/ && sudo ./install.sh
```

and open the printed address (`http://<machine>:8093/#setup`). NAS systems
(Synology Container Manager, QNAP, ZimaOS, Unraid): use
`deploy/docker-compose.yml` (image `ghcr.io/tomonwheels/amberimprove`).

## Commands

```
amberimprove [-config FILE] run            service: waits for the time window, then works title by title
amberimprove [-config FILE] once PATH      improve exactly one title (PATH relative to music)
amberimprove [-config FILE] plan           show the queue, change nothing
amberimprove [-config FILE] status         state of the night run
amberimprove [-config FILE] serve [-listen :8093]   status + setup page (German / English)
amberimprove [-config FILE] all   [-listen :8093]   both in one process (containers)
amberimprove version
```

Configuration: `/etc/amberimprove/config.json` — best set up on the status
page under **Setup** (drive, music folder, source of the originals, quality,
passes, time window, playback check, timing test). See also
`deploy/config.example.json`. The service re-reads the file before every night,
so changes apply without a restart.

systemd units: `deploy/amberimprove.service` (the worker) and
`deploy/amberimprove-web.service` (status + setup page, port 8093).

## Requirements and limits

- A **Linux** machine with the music drive **physically attached** (USB,
  SATA, NVMe). Over a network share (SMB/NFS) the timed writes end at the
  network client and never reach the medium; the setup refuses such a folder.
  NAS users run amberIMPROVE on the NAS itself (if it allows) or attach the
  drive to a small Linux box. The *originals* may come over the network.
- The Linux machine can be a NUC, Raspberry Pi, NAS
  with Linux, or any PC booted from a stick). bufhrt is built on that machine
  (`gcc`, `make`, `git`). macOS and Windows are not supported: bufhrt is a
  Linux program.
- The originals: **the file itself** (no second copy needed, verified bit by
  bit), a second copy in a local/mounted folder, or another machine via
  rsync over ssh.
- Playback check: any HTTP URL returning JSON with a state field (e.g.
  amberPLAY). Players without such a URL are not detected — then only the time
  window protects.
- The status page runs as root and can change the configuration: keep it in
  your home network. bufhrt's path and extra ssh options can only be changed in
  the file, never through the page.

## bufhrt

amberIMPROVE uses its own bufhrt, built from frankl's unmodified sources:

```
sudo scripts/build-frankl.sh v0.9.3     # → /opt/amberimprove/bin/bufhrt
```

Choose the bufhrt parameters so that your machine keeps the timing: run a pass
and look at the recorded `delayed_blocks` — 0 means every block was written on
time. frankl's very slow set (10 loops/s, 188044 bytes/s, 256 out-copies) keeps
perfect timing even on slow CPUs, but takes long.

## Build

```
go build -o amberimprove ./cmd/amberimprove
GOOS=linux GOARCH=amd64 go build -o amberimprove ./cmd/amberimprove
```

## Credits and license

**GPL-3.0-or-later.** This program is free software.

- The procedure is **frankl's** (`improvefile` / `bufhrt` from his stereo
  utilities, GPL-3.0-or-later): <https://github.com/frankl-audio/frankl_stereo>.
  amberIMPROVE runs bufhrt as a separate program.
- The pass structure with a bit-identity check after every pass follows the
  **nsc scripts by Harald Scherer** (nihil.sine.causa, GPL-3.0-or-later):
  <https://github.com/nihilsinecausa/nsc>.

Both are discussed at length on aktives-hoeren.de. amberIMPROVE lives in its own
public repository, separate from the proprietary amberSUITE; the suite only
reads the `.amberimprove.json` files (data).

See `LICENSE` for the full GNU General Public License v3.
