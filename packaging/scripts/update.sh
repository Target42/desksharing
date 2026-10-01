#!/bin/sh
# Installiert ein neues Desksharing-.deb, setzt die Rechte der Mobile-Artefakte
# und startet den Dienst neu. /etc/desksharing/desksharing.env bleibt erhalten.
set -eu

if [ "$(id -u)" -ne 0 ]; then
	echo "Als root ausführen: sudo desksharing-update paket.deb" >&2
	exit 1
fi
if [ $# -ne 1 ] || [ ! -f "$1" ]; then
	echo "Aufruf: desksharing-update desksharing_<version>_amd64.deb" >&2
	exit 1
fi

dpkg -i "$1"
desksharing-fix-mobile-perms
if command -v systemctl >/dev/null 2>&1; then
	systemctl daemon-reload
	systemctl restart desksharing
	systemctl --no-pager --full status desksharing
fi
