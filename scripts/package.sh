#!/bin/bash
# Builds the release packages dist/amberimprove-<version>-linux-{amd64,arm64}.tar.gz
#
# Input: frankl's two programs, statically linked, built on a machine of that
# architecture (gcc with glibc — improvedirect does not compile against musl:
# it includes both <linux/prctl.h> and <sys/prctl.h>, tried 13.11.2026):
#
#   build/bufhrt/bufhrt-linux-<arch>
#     git clone https://github.com/frankl-audio/frankl_stereo && git checkout v0.9.3
#     make REFRESH=X8664 CC="gcc -static" bin/bufhrt     (amd64)
#     make REFRESH=AA64  CC="gcc -static" bin/bufhrt     (arm64)
#
#   build/improvedirect/improvedirect-linux-<arch>      (since 0.2.0)
#     git checkout v0.9.4 && make clean
#     make REFRESH=X8664 CC="gcc -static" bin/improvedirect     (amd64)
#     make REFRESH=AA64  CC="gcc -static" bin/improvedirect     (arm64)
set -euo pipefail
cd "$(dirname "$0")/.."
V="$(cat VERSION)"
rm -rf dist && mkdir -p dist
for ARCH in amd64 arm64; do
  B="build/bufhrt/bufhrt-linux-$ARCH"
  I="build/improvedirect/improvedirect-linux-$ARCH"
  for f in "$B" "$I"; do
    [ -f "$f" ] || { echo "fehlt: $f" >&2; exit 1; }
  done
  N="amberimprove-$V-linux-$ARCH"
  D="dist/$N"
  mkdir -p "$D/bin" "$D/systemd"
  GOOS=linux GOARCH=$ARCH CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$V" -o "$D/bin/amberimprove" ./cmd/amberimprove
  install -m 755 "$B" "$D/bin/bufhrt"
  install -m 755 "$I" "$D/bin/improvedirect"
  install -m 644 deploy/amberimprove.service deploy/amberimprove-web.service "$D/systemd/"
  install -m 755 scripts/install.sh scripts/uninstall.sh "$D/"
  install -m 644 deploy/config.example.json deploy/docker-compose.yml README.md CHANGELOG.md LICENSE "$D/"
  for f in docs/ANLEITUNG*.md; do [ -f "$f" ] && install -m 644 "$f" "$D/"; done
  echo "$ARCH" > "$D/ARCH"
  cat > "$D/QUELLEN.txt" <<Q
amberIMPROVE $V — GPL-3.0-or-later
Quelltext:        https://github.com/tomonwheels/amberimprove
bin/bufhrt:       frankl's stereo utilities v0.9.3, unverändert, statisch gebaut
                  https://github.com/frankl-audio/frankl_stereo (Tag v0.9.3,
                  Commit f41973039f49ae6b6e3f729691c4cfce2bad9da0)
bin/improvedirect: frankl's stereo utilities v0.9.4, unverändert, statisch gebaut
                  https://github.com/frankl-audio/frankl_stereo (Tag v0.9.4,
                  Commit df48764f32fd48b2576afbefe40b13d1e4f498e9)
Beide Programme: Copyright frankl, GPL-3.0-or-later (siehe LICENSE).
Q
  COPYFILE_DISABLE=1 tar -C dist -czf "dist/$N.tar.gz" "$N"
  rm -rf "$D"
  echo "dist/$N.tar.gz"
done
cp deploy/docker-compose.yml dist/
(cd dist && shasum -a 256 *.tar.gz docker-compose.yml > SHA256SUMS && cat SHA256SUMS)
