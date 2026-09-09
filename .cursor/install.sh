#!/usr/bin/env bash
# Idempotent bootstrap for the Kelmor local dev preview.
# Durable setup only: system packages, compiled binaries, portal deps, and the
# control database. Per-boot process startup lives in .cursor/start.sh and the
# environment terminals.
set -euo pipefail
cd "$(dirname "$0")/.."

PG_VERSION="16"
RUN_USER="$(id -un)"

echo "==> Ensuring PostgreSQL is installed"
if ! command -v psql >/dev/null 2>&1; then
	sudo apt-get update -qq
	sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
		postgresql postgresql-contrib
fi

echo "==> Building Go control-plane binaries"
go mod download
make build

echo "==> Installing portal (Vite SPA) dependencies"
npm install --prefix portals/server
npm install --prefix portals/account

echo "==> Provisioning the control database (panel_control)"
sudo pg_ctlcluster "$PG_VERSION" main start 2>/dev/null || true
for _ in $(seq 1 30); do
	[[ -S /var/run/postgresql/.s.PGSQL.5432 ]] && break
	sleep 1
done

# panel-dev connects over the unix socket with peer auth as the runtime user,
# so it needs a matching role. Superuser lets migration 000001 create pgcrypto.
if ! sudo -u postgres psql -tAc \
	"SELECT 1 FROM pg_roles WHERE rolname='${RUN_USER}'" | grep -q 1; then
	sudo -u postgres psql -c "CREATE ROLE \"${RUN_USER}\" LOGIN SUPERUSER"
fi

if ! sudo -u postgres psql -tAc \
	"SELECT 1 FROM pg_database WHERE datname='panel_control'" | grep -q 1; then
	sudo -u postgres createdb -O "${RUN_USER}" panel_control
fi

sudo -u postgres psql -d panel_control -c "CREATE EXTENSION IF NOT EXISTS pgcrypto" >/dev/null

echo "==> Install complete"
