# Kelmor

A desired-state hosting operating platform for **Ubuntu 24.04 LTS**. Product identity is kelmor.host; never use that as a customer hosting domain. Components:

- **Kelmor Director** — provider / server-admin control plane (was Server Portal)
- **Kelmor Control** — tenant self-serve portal (was Account Portal)
- **Kelmor Agent** — root-owned typed ops over `/run/panel/agent.sock` (`panel-agent` / `kelmor-agent`)
- API, CLI, installer, and worker (`panel-*` names remain; `kelmor-*` aliases are installed beside them)

The control plane is written in Go. Portals never write Nginx, `/etc/passwd`, or systemd units. Package `email_daily_limit` is enforced by `panel-smtp-policy` on the Postfix policy port. They call the API, which records desired state and durable jobs. Workers call a typed privileged agent. Director and Control navigation and write actions are hidden unless `/api/v1/me` lists that capability — the API still enforces the same checks. Package limits (domains, mailboxes, databases, cron, applications, FTP users) are enforced on create. File writes and SFTP sessions honor `disk_bytes` even if the kernel has no usrquota: the agent persists the cap, rejects over-quota `ApplyFile`, and switches the tenant to `internal-sftp -R` plus write-locked home dirs. Monthly transfer is summed from each site's nginx access log (`$body_bytes_sent` for the current calendar month) and stored on `resource_usage.bandwidth_bytes`. When that total reaches `bandwidth_bytes_monthly`, the agent rewrites the vhost to HTTP 509 while keeping the ACME challenge location. Package `concurrent_web_requests` is applied as nginx `limit_conn` on `$panel_account` (HTTP 429 when exceeded) and as PHP-FPM `pm.max_children`. Package `process_limit`, `io_weight`, and `iops` are written to the account systemd slice and, when the cgroup v2 `io` controller is available, to `io.weight` / `io.max`. Each mail domain gets a 2048-bit DKIM key under `/var/lib/panel/dkim`, a `default._domainkey` TXT record, and rspamd `dkim_signing` plus Postfix milter `127.0.0.1:11332`. Mail catch-all is `reject`, `discard`, or a mailbox local part, applied through Postfix virtual maps. DNSSEC is `pdnsutil secure-zone` on the bind backend (`bind-dnssec-db`); DS records are shown for the registrar. Alias domains (`type=alias`) share the primary site document root and are added to that vhost `server_name` list while keeping their own DNS zone. WordPress is installed through `POST /accounts/{id}/wordpress`: the worker creates a MariaDB and the agent unpacks core plus `wp-config.php` into the site document root (official tarball on a live host; fixture archive in tests). Virtual FTP users (vsftpd + pam_pwdfile) map to the Linux account and chroot to `public_html`. Encrypted HPM1 backups restore the home tree, MariaDB/PostgreSQL dumps, and mailbox Maildirs.

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
./scripts/ui-mvp.sh          # Chrome: provision → files/mail/backups → suspend → migrate → audit
```

- API: `http://127.0.0.1:18080`
- Kelmor Director (installed): `https://127.0.0.1:8443` — `admin` / `ChangeMeOnce!2026` (self-signed portal cert)
- Kelmor Control (installed): `https://127.0.0.1:8444` — sign in as a provisioned account username
- Vite HMR (optional): `18443` / `18444`

Default admin password is for development only. Change it before any real host.

## Production install (Ubuntu 24.04)

```bash
# verify installer signature, then:
sudo ./panel-install --hostname panel.example.net --admin-email ops@example.net --non-interactive
```

The installer writes `/var/lib/panel/install-state.json` and resumes failed phases. It does not require Docker for the control plane. A live install fails unless built Director/Control SPAs are present (`make portals` or the Debian package). nginx serves them on **8443** and **8444**, proxying `/api` and `/healthz` to the control API.

On a host with systemd as PID 1, the control plane and `panel-object-store` start only as systemd units (no forked duplicates). After install, `scripts/fresh-provision-smoke.sh` covers login → provision → HTTP/PHP isolation → mailbox → local backup → suspend → audit. Full path: `scripts/live-e2e.sh`.

## Layout

See `cmd/`, `internal/`, `agent/`, `db/migrations/`, `portals/`, `installer/`, `api/openapi.yaml`, and `docs/` (install, mail, DNS, CLI, backup, migration, security, **validation**).

Lab VM / DNS / SMTP relay credentials live in **`.run/validation/`** (gitignored). Copy `docs/validation/*.example` there, or `source scripts/load-validation-env.sh`. `panel-install` uses `TEST_DOMAIN` and `VM_PUBLIC_IPV4` when `--hostname` is omitted, and configures a Postfix SASL relay from `smtp.env`. SSH: `scripts/remote-vm.sh`. Details: `docs/validation.md`.

## Tests

```bash
make lint
make test
make test-security
make test-provisioning
./scripts/mvp-accept.sh   # live host: installer → API → CLI → portals
```

Encrypted account backups (HPM1) and native export/import are documented in `docs/backup.md` and `docs/migration.md`.

## Ports

21 (FTP) + 40000–40100 (PASV), 22, 25, 53, 80, 443, 587, 993, **8443** (Kelmor Director), **8444** (Kelmor Control), **19090** (loopback S3 object store). Development preview uses 18443/18444/18080.

MVP gap vs `main`: see `docs/kelmor-mvp-gap.md`.

## What is not in this first slice

Copied cPanel/WHM UI or trademarks, billing/WHMCS, Windows hosting, Kubernetes control, and any arbitrary root-shell API.
