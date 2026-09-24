#!/bin/bash
# amberIMPROVE — Installation (GPL-3.0-or-later).
#
#   sudo ./install.sh
#
# Installs the program and frankl's bufhrt to /opt/amberimprove, the two
# systemd services, and starts them. The configuration is made afterwards on
# the setup page in the browser (the address is printed at the end).
# An existing configuration (/etc/amberimprove) and all album records stay.
set -euo pipefail
cd "$(dirname "$0")"

die() { echo "FEHLER: $*" >&2; exit 1; }
[ "$(id -u)" = 0 ] || die "bitte mit sudo starten: sudo ./install.sh"
[ "$(uname -s)" = Linux ] || die "amberIMPROVE läuft nur unter Linux"
command -v systemctl >/dev/null || die "systemd fehlt — für Docker/NAS gibt es docker-compose.yml"

want="$(cat ARCH)"
case "$(uname -m)" in
  x86_64) have=amd64 ;;
  aarch64|arm64) have=arm64 ;;
  *) die "Rechnertyp $(uname -m) wird nicht unterstützt (nur x86_64 und aarch64)" ;;
esac
[ "$want" = "$have" ] || die "dieses Paket ist für $want, der Rechner ist $have — bitte das $have-Paket nehmen"

for tool in lsblk taskset; do
  command -v "$tool" >/dev/null || echo "Hinweis: $tool fehlt (Paket util-linux)"
done
command -v rsync >/dev/null || echo "Hinweis: rsync fehlt — nur nötig, wenn die Originale von einem anderen Rechner kommen"

echo "== Programme nach /opt/amberimprove/bin"
install -d -m 755 /opt/amberimprove/bin /etc/amberimprove /var/lib/amberimprove
install -m 755 bin/amberimprove /opt/amberimprove/bin/amberimprove.neu
mv /opt/amberimprove/bin/amberimprove.neu /opt/amberimprove/bin/amberimprove
install -m 755 bin/bufhrt /opt/amberimprove/bin/bufhrt.neu
mv /opt/amberimprove/bin/bufhrt.neu /opt/amberimprove/bin/bufhrt
/opt/amberimprove/bin/bufhrt --version

echo "== Dienste"
install -m 644 systemd/amberimprove.service /etc/systemd/system/amberimprove.service
install -m 644 systemd/amberimprove-web.service /etc/systemd/system/amberimprove-web.service
systemctl daemon-reload
systemctl enable amberimprove-web amberimprove >/dev/null 2>&1
systemctl restart amberimprove-web amberimprove
sleep 2
systemctl is-active --quiet amberimprove-web || die "die Statusseite startet nicht: journalctl -u amberimprove-web"

ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Fertig. Jetzt im Browser einrichten:"
echo "    http://${ip:-<dieser-rechner>}:8093/#setup"
echo
echo "Deinstallieren: sudo ./uninstall.sh"
