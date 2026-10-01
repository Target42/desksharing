#!/bin/sh
# Setzt Besitzer und Rechte nach dem Ablegen neuer Mobile-App-Artefakte
# unter /var/lib/desksharing/mobile.
set -eu

if [ "$(id -u)" -ne 0 ]; then
	echo "Als root ausführen: sudo desksharing-fix-mobile-perms" >&2
	exit 1
fi

dir=/var/lib/desksharing/mobile
install -d -o desksharing -g desksharing -m 750 "$dir"
chown -R desksharing:desksharing "$dir"
find "$dir" -type d -exec chmod 750 {} +
find "$dir" -type f -exec chmod 640 {} +
echo "Rechte gesetzt: $dir (desksharing:desksharing, Verzeichnisse 750, Dateien 640)"
