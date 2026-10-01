#!/bin/sh
# Legt die lokale PostgreSQL-Rolle und Datenbank an und schreibt DATABASE_URL
# nach /etc/desksharing/desksharing.env. Fragt fehlende Produktionswerte ab
# und startet den Dienst, sobald die Datei vollständig ist.
set -eu

ENV_FILE=/etc/desksharing/desksharing.env
DB_USER=desksharing
DB_NAME=desksharing

if [ "$(id -u)" -ne 0 ]; then
	echo "Als root ausführen: sudo desksharing-setup-database" >&2
	exit 1
fi
if [ ! -f "$ENV_FILE" ]; then
	echo "Konfiguration fehlt: $ENV_FILE" >&2
	exit 1
fi

get_kv() {
	key=$1
	awk -v k="$key" '
		index($0, k "=") == 1 {
			v = substr($0, length(k) + 2)
			if (v ~ /^".*"$/) v = substr(v, 2, length(v) - 2)
			print v
			exit
		}
	' "$ENV_FILE"
}

set_kv() {
	key=$1
	value=$2
	quoted=$(printf "%s" "$value" | sed 's/\\/\\\\/g; s/"/\\"/g')
	tmp=$(mktemp)
	KEY=$key VAL="\"$quoted\"" awk '
		index($0, ENVIRON["KEY"] "=") == 1 {
			print ENVIRON["KEY"] "=" ENVIRON["VAL"]
			found = 1
			next
		}
		{ print }
		END {
			if (!found) print ENVIRON["KEY"] "=" ENVIRON["VAL"]
		}
	' "$ENV_FILE" >"$tmp"
	cat "$tmp" >"$ENV_FILE"
	rm -f "$tmp"
	chown root:desksharing "$ENV_FILE"
	chmod 640 "$ENV_FILE"
}

prompt() {
	label=$1
	current=$2
	secret=${3:-}
	if [ "${DESKSHARING_NONINTERACTIVE:-}" = "1" ]; then
		echo "Pflichtwert fehlt oder ist ein Platzhalter: $label" >&2
		exit 1
	fi
	if [ -n "$secret" ]; then
		printf "%s: " "$label" >/dev/tty
		stty -echo </dev/tty
		IFS= read -r value </dev/tty || value=
		stty echo </dev/tty
		printf "\n" >/dev/tty
	else
		printf "%s [%s]: " "$label" "$current" >/dev/tty
		IFS= read -r value </dev/tty || value=
		if [ -z "$value" ]; then
			value=$current
		fi
	fi
	printf "%s" "$value"
}

weak_password() {
	pw=$1
	email=${2:-}
	bytes=$(printf "%s" "$pw" | wc -c)
	if [ "$bytes" -lt 12 ] || [ "$bytes" -gt 72 ]; then
		return 0
	fi
	lower=$(printf "%s" "$pw" | tr '[:upper:]' '[:lower:]')
	case $lower in
	password1234 | passwort1234 | changeme1234 | changeme123 | desksharing1 | desksharing | administrator | qwertyuiopas | welcome12345 | letmein12345 | admin1234567)
		return 0
		;;
	esac
	if [ -n "$email" ] && [ "$lower" = "$(printf "%s" "$email" | tr '[:upper:]' '[:lower:]')" ]; then
		return 0
	fi
	return 1
}

if ! command -v psql >/dev/null 2>&1 || ! command -v openssl >/dev/null 2>&1; then
	apt-get update
	apt-get install -y postgresql openssl
fi
if command -v systemctl >/dev/null 2>&1; then
	systemctl enable --now postgresql
fi

ssl=
i=0
while [ "$i" -lt 15 ]; do
	ssl=$(runuser -u postgres -- psql -tAc "SHOW ssl;" 2>/dev/null | tr -d '[:space:]' || true)
	if [ -n "$ssl" ]; then
		break
	fi
	i=$((i + 1))
	sleep 1
done
if [ "$ssl" != "on" ]; then
	echo "PostgreSQL-SSL ist aus. Für APP_ENV=production muss sslmode=require funktionieren." >&2
	echo "In postgresql.conf ssl=on setzen (Debian: ssl-cert / Snakeoil) und PostgreSQL neu laden." >&2
	exit 1
fi

password=$(get_kv POSTGRES_PASSWORD)
case $password in
"" | CHANGE_ME | desksharing | postgres | password | changeme | changeme123)
	password=$(openssl rand -hex 24)
	;;
esac

password_sql=$(printf "%s" "$password" | sed "s/'/''/g")
runuser -u postgres -- psql -v ON_ERROR_STOP=1 <<SQL
DO \$\$
BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${DB_USER}') THEN
		CREATE ROLE ${DB_USER} LOGIN PASSWORD '${password_sql}';
	ELSE
		ALTER ROLE ${DB_USER} PASSWORD '${password_sql}';
	END IF;
END
\$\$;
SQL

exists=$(runuser -u postgres -- psql -tAc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" | tr -d '[:space:]')
if [ "$exists" != "1" ]; then
	runuser -u postgres -- createdb --owner="$DB_USER" "$DB_NAME"
fi

database_url="postgres://${DB_USER}:${password}@127.0.0.1:5432/${DB_NAME}?sslmode=require"
set_kv POSTGRES_PASSWORD "$password"
set_kv DATABASE_URL "$database_url"
set_kv POSTGRES_HOST "127.0.0.1"
set_kv POSTGRES_PORT "5432"
set_kv POSTGRES_USER "$DB_USER"
set_kv POSTGRES_DB "$DB_NAME"
set_kv POSTGRES_SSLMODE "require"

base=$(get_kv PUBLIC_BASE_URL)
case $base in
https://*) ;;
*)
	base=$(prompt "Öffentliche HTTPS-Adresse" "https://")
	;;
esac
case $base in
*example.com* | https:// | "")
	base=$(prompt "Öffentliche HTTPS-Adresse (kein example.com)" "$base")
	;;
esac
case $base in
https://*example.com* | https:// | "")
	echo "PUBLIC_BASE_URL muss https:// und die echte Adresse sein, nicht example.com." >&2
	exit 1
	;;
https://*)
	set_kv PUBLIC_BASE_URL "${base%/}"
	;;
*)
	echo "PUBLIC_BASE_URL muss mit https:// beginnen." >&2
	exit 1
	;;
esac

admin_email=$(get_kv PLATFORM_ADMIN_EMAIL)
case $admin_email in
"" | admin@example.com)
	admin_email=$(prompt "E-Mail des Platform-Admins" "")
	;;
esac
case $admin_email in
*@*)
	set_kv PLATFORM_ADMIN_EMAIL "$admin_email"
	;;
*)
	echo "PLATFORM_ADMIN_EMAIL muss eine E-Mail-Adresse sein." >&2
	exit 1
	;;
esac

admin_password=$(get_kv PLATFORM_ADMIN_PASSWORD)
if weak_password "$admin_password" "$admin_email" || [ "$admin_password" = "CHANGE_ME" ]; then
	admin_password=$(prompt "Passwort des Platform-Admins (min. 12 Zeichen)" "" secret)
	if weak_password "$admin_password" "$admin_email"; then
		echo "Platform-Admin-Passwort ist zu kurz, zu lang, zu üblich oder identisch mit der E-Mail." >&2
		exit 1
	fi
	set_kv PLATFORM_ADMIN_PASSWORD "$admin_password"
fi

smtp_host=$(get_kv SMTP_HOST)
if [ -z "$smtp_host" ]; then
	smtp_host=$(prompt "SMTP-Host" "")
	set_kv SMTP_HOST "$smtp_host"
fi
smtp_from=$(get_kv SMTP_FROM)
if [ -z "$smtp_from" ]; then
	smtp_from=$(prompt "SMTP-Absender, z. B. Desksharing <noreply@firma.de>" "")
	set_kv SMTP_FROM "$smtp_from"
fi
smtp_user=$(get_kv SMTP_USER)
if [ -z "$smtp_user" ]; then
	smtp_user=$(prompt "SMTP-Benutzer (leer lassen, wenn keiner nötig ist)" "")
	set_kv SMTP_USER "$smtp_user"
fi
smtp_password=$(get_kv SMTP_PASSWORD)
if [ -z "$smtp_password" ] && [ -n "$smtp_user" ]; then
	smtp_password=$(prompt "SMTP-Passwort" "" secret)
	set_kv SMTP_PASSWORD "$smtp_password"
fi
if [ -z "$smtp_host" ] || [ -z "$smtp_from" ]; then
	echo "SMTP_HOST und SMTP_FROM sind in Produktion Pflicht." >&2
	exit 1
fi

if command -v systemctl >/dev/null 2>&1; then
	systemctl daemon-reload
	systemctl enable desksharing
	systemctl restart desksharing
	echo "Dienst desksharing gestartet. Status: systemctl status desksharing"
fi
echo "Nginx: server_name in /etc/nginx/sites-available/desksharing setzen, Site aktivieren, Zertifikat holen."
echo "Datenbankpasswort steht in $ENV_FILE."
