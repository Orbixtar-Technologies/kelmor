#!/usr/bin/env bash
# Per-boot startup for the Kelmor local dev preview.
# Brings up PostgreSQL and waits for its socket. The API and portals run as
# environment terminals (see .cursor/environment.json) once this succeeds.
set -euo pipefail

PG_VERSION="16"

sudo pg_ctlcluster "$PG_VERSION" main start 2>/dev/null || true

for _ in $(seq 1 30); do
	if [[ -S /var/run/postgresql/.s.PGSQL.5432 ]]; then
		echo "PostgreSQL is ready on /var/run/postgresql"
		exit 0
	fi
	sleep 1
done

echo "PostgreSQL did not become ready in time" >&2
exit 1
