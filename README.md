# Hosting Panel

A desired-state hosting operating platform for **Ubuntu 24.04 LTS**. It exposes two distinct applications:

- **Server Portal** — host, reseller and package administration
- **Account Portal** — tenant websites, DNS, mail, files and backups

The control plane is written in Go. Portals never write Nginx, `/etc/passwd`, or systemd units. They call the API, which records desired state and durable jobs. Workers call a typed privileged agent.

This repository is the first production slice of that architecture: schema, auth/RBAC/audit, job engine, agent operations, both portals, CLI, and a resumable installer. Full Ubuntu service installation (Postfix, PowerDNS, MariaDB, …) is orchestrated by installer phases and is intended to run on a clean 24.04 host, not inside a containerized control plane.

## Local preview (this environment)

The control plane requires PostgreSQL (`panel_control`). Local peer auth:

```bash
sudo pg_ctlcluster 16 main start   # if the cluster is not running
createdb panel_control             # once
```

```bash
make build
sudo ./scripts/apply-host-stack.sh   # Postfix, Dovecot, PowerDNS, nginx, privileged agent
PANEL_DEV=1 PANEL_AGENT_SOCK=/run/panel/agent.sock \
  PANEL_DATABASE_URL=postgres:///panel_control?host=/var/run/postgresql \
  PANEL_STATE_DIR=$PWD/var/panel PANEL_PUBLIC_IPV4=127.0.0.1 \
  ./dist/bin/panel-dev
# other terminals
cd portals/server && npm install && npm run dev
cd portals/account && npm install && npm run dev
./scripts/live-e2e.sh
```

- API: `http://127.0.0.1:18080`
- Server Portal: `http://127.0.0.1:18443` — `admin` / `ChangeMeOnce!2026`
- Account Portal: `http://127.0.0.1:18444` — sign in as a provisioned account username

Default admin password is for development only. Change it before any real host.

## Production install (Ubuntu 24.04)

```bash
# verify installer signature, then:
sudo ./panel-install --hostname panel.example.net --admin-email ops@example.net --non-interactive
```

The installer writes `/var/lib/panel/install-state.json` and resumes failed phases. It does not require Docker for the control plane.

## Layout

See `cmd/`, `internal/`, `agent/`, `db/migrations/`, `portals/`, `installer/`, `api/openapi.yaml`, and `docs/` (install, mail, DNS, CLI, backup, migration, security).

## Tests

```bash
make lint
make test
make test-security
make test-provisioning
```

Encrypted account backups (HPM1) and native export/import are documented in `docs/backup.md` and `docs/migration.md`.

## Ports

22, 25, 53, 80, 443, 587, 993, **8443** (Server Portal), **8444** (Account Portal). Development preview uses 18443/18444/18080.

## What is not in this first slice

Copied cPanel/WHM UI or trademarks, billing/WHMCS, Windows hosting, Kubernetes control, and any arbitrary root-shell API.
