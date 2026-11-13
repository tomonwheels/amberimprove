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
   onto the medium, once or several times (default 1). After each pass the file is
   flushed, dropped from the page cache, read back from the medium and compared
   (SHA-256). Up to 10 retries per pass. bufhrt's `delayed block` messages are
   counted and recorded: they show how evenly the pass was written.
   Instead of bufhrt, frankl's `improvedirect` can write the passes — see
   [Two procedures](#two-procedures).
5. **Replace:** the last pass file is renamed onto the original name (same
   filesystem, metadata only) with the original's time, owner and mode.
6. **Record:** the entry goes into the album's `.amberimprove.json`.

## Two procedures

Since 0.2.0 the configuration (`"procedure"`, or **Procedure** on the setup
page) chooses which of frankl's programs writes a pass. Everything around it
is the same for both: guards and time window, bit-identity check after every
pass, retries, replacement by rename, and a title that is interrupted stays
exactly as it was and is started anew the next night.

| | `bufhrt` (default) | `direct` |
|---|---|---|
| Program | `bufhrt --interval` (frankl_stereo v0.9.3) | `improvedirect` (frankl_stereo v0.9.4) |
| Data path | original in RAM, written from there | small blocks read and written in turns, straight from and to the medium (`O_DIRECT`, no page cache) |
| Pass 1 reads | the original in RAM | the title on the medium (verified against the original first) |
| frankl's set | 10 loops/s, 188044 bytes/s, 256 out-copies | `-o -i -w`, 10 loops/s, 40960 bytes/s (one 4096-byte page per loop), fdatasync every 10 loops, 128 copies |
| Pure writing time | about 5.6 s per MiB | about 25.6 s per MiB |
| Recorded timing misses | `delayed block` messages | `timer reset` count of `improvedirect -v` |

`improvedirect` is frankl's newer program: it "reads and writes small blocks in
turns, instead of loading music data into RAM first" (frankl_stereo `CHANGES`,
0.9.3 to 0.9.4). The default set is the line from his `scripts/improvefile`
("very good but slow"):

```
improvedirect -v -o -i -w "$1" "$2" 10 40960 10 128
```

It is about 4.6 times slower than bufhrt's very slow set, so far fewer titles
fit into a night, and a title must fit into one window. The drive must allow
`O_DIRECT` (the setup check asks), and the service must run as root or own the
music files (`improvedirect` opens them with `O_NOATIME`). The times above are
derived from the parameters; the timing test on the setup page measures a
1 MB sample on your machine.

Changing the procedure changes the recorded method name
(`frankl-0.9.4/direkt/…` instead of `frankl-0.9.3/…`): titles improved with
the other procedure count as "older method" and are written again. All
parameters of the `direct` block in `deploy/config.example.json` can be set by
hand; the album record lists them under `params` next to `"procedure"`.

## Guides (German)

- [docs/ANLEITUNG-STANDALONE.md](docs/ANLEITUNG-STANDALONE.md) — install and
  set up on any Linux machine, plus Synology/UGREEN/NAS via container
- [docs/ANLEITUNG-AMBERSUITE.md](docs/ANLEITUNG-AMBERSUITE.md) — together with
  amberSUITE (wand in amberPLAY, playback check)

## Install

Download the package for your machine from the releases page, then:

```
tar -xzf amberimprove-*-linux-*.tar.gz && cd amberimprove-*/ && sudo ./install.sh
```

and open the printed address (`http://<machine>:8093/#setup`). NAS systems
(Synology Container Manager, UGREEN UGOS Pro, QNAP, ZimaOS, Unraid): use
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
  with Linux, or any PC booted from a stick). The release packages bring
  bufhrt and improvedirect along; from source they are built on that machine
  (`gcc`, `make`, `git`). macOS and Windows are not supported: both are
  Linux programs.
- The originals: **the file itself** (no second copy needed, verified bit by
  bit), a second copy in a local/mounted folder, or another machine via
  rsync over ssh.
- Playback check: any HTTP URL returning JSON with a state field (e.g.
  amberPLAY). Players without such a URL are not detected — then only the time
  window protects.
- The status page runs as root and can change the configuration: keep it in
  your home network. The paths of bufhrt and improvedirect and extra ssh
  options can only be changed in the file, never through the page.

## bufhrt and improvedirect

amberIMPROVE uses its own bufhrt and improvedirect, built from frankl's
unmodified sources:

```
sudo scripts/build-frankl.sh     # → /opt/amberimprove/bin/bufhrt         (v0.9.3)
                                 #   /opt/amberimprove/bin/improvedirect  (v0.9.4)
```

bufhrt stays at v0.9.3 (commit `f419730`), the version the recorded method
name `frankl-0.9.3/…` refers to; improvedirect comes from v0.9.4 (commit
`df48764`), the first release that contains it.

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

- The procedure is **frankl's** (`improvefile` with `bufhrt` and
  `improvedirect` from his stereo utilities, Copyright frankl,
  GPL-3.0-or-later): <https://github.com/frankl-audio/frankl_stereo>.
  amberIMPROVE runs both as separate, unmodified programs — bufhrt from tag
  v0.9.3 (commit `f41973039f49ae6b6e3f729691c4cfce2bad9da0`), improvedirect
  from tag v0.9.4 (commit `df48764f32fd48b2576afbefe40b13d1e4f498e9`). The
  release packages contain them statically linked; `QUELLEN.txt` in each
  package names the sources.
- The pass structure with a bit-identity check after every pass follows the
  **nsc scripts by Harald Scherer** (nihil.sine.causa, GPL-3.0-or-later):
  <https://github.com/nihilsinecausa/nsc>.

Both are discussed at length on aktives-hoeren.de. amberIMPROVE lives in its own
public repository, separate from the proprietary amberSUITE; the suite only
reads the `.amberimprove.json` files (data).

See `LICENSE` for the full GNU General Public License v3.
