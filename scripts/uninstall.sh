#!/bin/bash
# amberIMPROVE — Deinstallation.
#
#   sudo ./uninstall.sh           Programm und Dienste entfernen
#   sudo ./uninstall.sh --alles   zusätzlich Konfiguration und Zustand löschen
#
# Die Musikdateien und die .amberimprove.json in den Albumordnern bleiben
# immer, wie sie sind — improvte Dateien sind bit-identisch mit dem Original.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "bitte mit sudo starten" >&2; exit 1; }
systemctl disable --now amberimprove amberimprove-web >/dev/null 2>&1 || true
rm -f /etc/systemd/system/amberimprove.service /etc/systemd/system/amberimprove-web.service
systemctl daemon-reload
rm -rf /opt/amberimprove /dev/shm/amberimprove
if [ "${1:-}" = "--alles" ]; then
  rm -rf /etc/amberimprove /var/lib/amberimprove
  echo "amberIMPROVE samt Konfiguration entfernt."
else
  echo "amberIMPROVE entfernt. Konfiguration bleibt in /etc/amberimprove (löschen: --alles)."
fi
