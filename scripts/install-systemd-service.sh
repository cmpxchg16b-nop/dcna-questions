#!/bin/sh
# install-systemd-service.sh - install the exam-server systemd unit.
#
# The unit runs the ./server binary from this checkout with the checkout
# directory as its working directory. The server reads .env.local / .env
# from the working directory at startup, so JWT_SECRET & friends keep
# working exactly as they do when launching manually.
#
# Usage:
#   scripts/install-systemd-service.sh
#
# Optional environment overrides:
#   SERVICE_USER   user the service runs as (default: the invoking user)
#   UNIT_NAME      systemd unit name (default: exam-server.service)
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

UNIT_NAME=${UNIT_NAME:-exam-server.service}
UNIT_TEMPLATE=$SCRIPT_DIR/${UNIT_NAME%.service}.service
UNIT_DST=/etc/systemd/system/$UNIT_NAME
SERVICE_USER=${SERVICE_USER:-${SUDO_USER:-$(id -un)}}

fail() { echo "error: $*" >&2; exit 1; }
warn() { echo "warning: $*" >&2; }

command -v systemctl >/dev/null 2>&1 || fail "systemctl not found; this script requires systemd."
[ -f "$UNIT_TEMPLATE" ] || fail "unit template not found: $UNIT_TEMPLATE"
[ -x "$PROJECT_DIR/server" ] || fail "server binary missing or not executable: $PROJECT_DIR/server (build it first)"
[ -d "$PROJECT_DIR/assets" ] || fail "assets directory not found: $PROJECT_DIR/assets"
[ -d "$PROJECT_DIR/exams-dn42" ] || fail "exam directory not found: $PROJECT_DIR/exams-dn42"
[ -f "$PROJECT_DIR/serverConfig.xml" ] || fail "config not found: $PROJECT_DIR/serverConfig.xml"

# systemd splits ExecStart on whitespace and the unit file has no way to
# escape spaces in paths, so a checkout under such a path cannot work.
case $PROJECT_DIR in
	*[[:space:]]*) fail "project path contains whitespace: $PROJECT_DIR" ;;
esac

# A missing JWT secret is fatal at startup (see getJWTSecret in
# cmd/server/main.go), and the service environment will not inherit the
# invoking shell's variables - only the dotenv files count.
if [ ! -f "$PROJECT_DIR/.env" ] && [ ! -f "$PROJECT_DIR/.env.local" ]; then
	warn "no .env or .env.local in $PROJECT_DIR"
	warn "the server needs JWT_SECRET set (e.g. in .env) or it will refuse to start"
fi

SUDO=
if [ "$(id -u)" -ne 0 ]; then
	command -v sudo >/dev/null 2>&1 || fail "not running as root and sudo not found"
	SUDO=sudo
fi

echo "Installing $UNIT_NAME:"
echo "  project dir : $PROJECT_DIR"
echo "  service user: $SERVICE_USER"
echo "  destination : $UNIT_DST"

# Escape the sed replacement texts (& is special in replacements, | is the
# delimiter) so unusual-but-legal path characters survive substitution.
ESC_PROJECT_DIR=$(printf '%s' "$PROJECT_DIR" | sed 's/[&|\\]/\\&/g')
ESC_SERVICE_USER=$(printf '%s' "$SERVICE_USER" | sed 's/[&|\\]/\\&/g')

RENDERED=$(mktemp)
trap 'rm -f "$RENDERED"' EXIT
sed -e "s|@PROJECT_DIR@|$ESC_PROJECT_DIR|g" \
	-e "s|@SERVICE_USER@|$ESC_SERVICE_USER|g" \
	"$UNIT_TEMPLATE" >"$RENDERED"

$SUDO install -m 644 "$RENDERED" "$UNIT_DST"
$SUDO systemctl daemon-reload

SERVICE=${UNIT_NAME%.service}
echo
echo "Installed. Next steps:"
echo "  sudo systemctl enable --now $SERVICE   # start now and on boot"
echo "  sudo systemctl status $SERVICE"
echo "  journalctl -u $SERVICE -f              # follow logs"
