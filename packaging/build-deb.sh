#!/bin/sh
# Baut ein Debian-Paket. Auf Linux oder in WSL aus dem Repository-Wurzelverzeichnis:
#   sh packaging/build-deb.sh
#   sh packaging/build-deb.sh 0.1.0 arm64
set -eu

version=${1:-0.1.0}
arch=${2:-amd64}
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

case "$arch" in
amd64) goarch=amd64 ;;
arm64) goarch=arm64 ;;
*)
	echo "Architektur muss amd64 oder arm64 sein." >&2
	exit 1
	;;
esac

install -d "$stage/usr/bin" "$stage/DEBIAN" \
	"$stage/lib/systemd/system" \
	"$stage/etc/nginx/sites-available" \
	"$stage/etc/desksharing" \
	"$stage/usr/share/desksharing"

CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags "-s -w" \
	-o "$stage/usr/bin/desksharing" "$root/cmd/server"
chmod 755 "$stage/usr/bin/desksharing"
install -m 755 "$root/packaging/scripts/setup-database.sh" "$stage/usr/bin/desksharing-setup-database"
install -m 755 "$root/packaging/scripts/update.sh" "$stage/usr/bin/desksharing-update"
install -m 755 "$root/packaging/scripts/fix-mobile-perms.sh" "$stage/usr/bin/desksharing-fix-mobile-perms"
install -m 644 "$root/packaging/systemd/desksharing.service" "$stage/lib/systemd/system/desksharing.service"
install -m 644 "$root/packaging/nginx/desksharing.conf" "$stage/etc/nginx/sites-available/desksharing"
install -m 644 "$root/packaging/env/desksharing.env" "$stage/usr/share/desksharing/desksharing.env"
install -m 640 "$root/packaging/env/desksharing.env" "$stage/etc/desksharing/desksharing.env"

install -m 755 "$root/packaging/debian/postinst" "$stage/DEBIAN/postinst"
install -m 755 "$root/packaging/debian/prerm" "$stage/DEBIAN/prerm"
install -m 755 "$root/packaging/debian/postrm" "$stage/DEBIAN/postrm"
install -m 644 "$root/packaging/debian/conffiles" "$stage/DEBIAN/conffiles"

cat >"$stage/DEBIAN/control" <<EOF
Package: desksharing
Version: ${version}
Section: web
Priority: optional
Architecture: ${arch}
Depends: adduser
Recommends: postgresql, nginx
Maintainer: Desksharing <noreply@localhost>
Description: Desksharing server
 Web server for desk and resource booking.
 Configuration lives in /etc/desksharing/desksharing.env.
EOF

out="$root/dist/desksharing_${version}_${arch}.deb"
mkdir -p "$root/dist"
dpkg-deb --root-owner-group -Zgzip --build "$stage" "$out"
echo "Paket: $out"
