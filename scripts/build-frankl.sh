#!/bin/bash
# Builds bufhrt and improvedirect from frankl's stereo utilities (GPL-3.0-or-
# later) on the target machine and installs them to /opt/amberimprove/bin.
# amberIMPROVE uses its own programs, not copies that other software may have
# patched.
#
#   sudo scripts/build-frankl.sh [BUFHRT_TAG] [DIRECT_TAG]
#
# bufhrt stays at v0.9.3: the method name recorded in every album file says
# "frankl-0.9.3", and titles improved with it must not silently get a
# different program. improvedirect exists since v0.9.4 (13.11.2026).
set -euo pipefail

TAG="${1:-v0.9.3}"
DIRECT_TAG="${2:-v0.9.4}"
DEST=/opt/amberimprove/bin
SRC=/opt/amberimprove/src/frankl_stereo

case "$(uname -m)" in
  x86_64)  REFRESH=X8664 ;;
  aarch64) REFRESH=AA64 ;;
  *) echo "Unbekannte Architektur $(uname -m)"; exit 1 ;;
esac

mkdir -p "$DEST" "$(dirname "$SRC")"
if [ -d "$SRC/.git" ]; then
  git -C "$SRC" fetch --tags --quiet
else
  git clone --quiet https://github.com/frankl-audio/frankl_stereo.git "$SRC"
fi

# baue TAG PROGRAMM: one program from one tag, from a clean tree (the object
# files of the two tags must not mix).
baue() {
  git -C "$SRC" checkout --quiet "$1"
  make -C "$SRC" clean >/dev/null 2>&1 || true
  make -C "$SRC" REFRESH="$REFRESH" "bin/$2"
  install -m 0755 "$SRC/bin/$2" "$DEST/$2"
  echo "$2 $1 ($REFRESH) → $DEST/$2"
}

baue "$TAG" bufhrt
baue "$DIRECT_TAG" improvedirect
