#!/bin/bash
# Builds bufhrt from frankl's stereo utilities (GPL-3.0-or-later) on the target
# machine and installs it to /opt/amberimprove/bin. amberIMPROVE uses its own
# bufhrt, not a copy that other software may have patched.
set -euo pipefail

TAG="${1:-v0.9.3}"
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
git -C "$SRC" checkout --quiet "$TAG"
make -C "$SRC" clean >/dev/null 2>&1 || true
make -C "$SRC" REFRESH="$REFRESH" bin/bufhrt
install -m 0755 "$SRC/bin/bufhrt" "$DEST/bufhrt"
echo "bufhrt $TAG ($REFRESH) → $DEST/bufhrt"
