# Kelmor MVP gap (main @ e02bdfb)

Honest inventory of `main` against the product identity and the
fresh-Ubuntu MVP path. Status words mean:

| Status | Meaning |
| --- | --- |
| **VERIFIED** | Code path exists, is tested, and writes real artifacts (or live host ops when `Host.live()`). |
| **PARTIAL** | Desired-state / agent op exists, but a step is optional, stubbed, ignored-on-error, or not wired into account provision. |
| **MISSING** | Not implemented on the MVP path. |
| **MOCK** | Succeeds without touching the named subsystem (sandbox `Host.Root != ""`, or a recorded-only engine). |

This file is a durable map, not a promise that a later commit closed every row.
Re-verify against `git log` before treating a row as current.

## Identity

Kelmor is a self-hosted hosting *operating platform* (cPanel/WHM/Plesk
scope), not a dashboard template.

| Surface | Target name | On `main` before this work |
| --- | --- | --- |
| Provider / server admin UI | **Kelmor Director** | "Server Portal" |
| Tenant UI | **Kelmor Control** | "Account Portal" |
| Root-owned typed ops | **Kelmor Agent** | `panel-agent` on `/run/panel/agent.sock` |
| API / CLI / installer / worker | one brand | `panel-*` binaries, "Hosting Panel" titles |
| Product domain | kelmor.host (identity only) | used in lab validation examples; not hardcoded as a customer site domain |

Privilege zones (do not flatten):

| Zone | Role | On-disk / process contract |
| --- | --- | --- |
| **A** | Unprivileged control plane | `panel-api`, `panel-worker`, Director/Control SPAs; refuse root unless `PANEL_ALLOW_ROOT=1` |
| **B** | Root-owned typed agent | `panel-agent` / UDS `/run/panel/agent.sock`; allow-listed methods only; no `exec(string)` RPC |
| **C** | Tenant workloads | dedicated Linux users UID/GID ≥ 20000, nginx/php-fpm/mail as that identity |

**Stable on purpose (privilege model):** `/var/lib/panel`, `/run/panel`,
`/etc/panel`, system user `panel`, `PANEL_*` environment variables, nftables
`table inet panel`, Go module `github.com/hosting-panel/panel`. Renaming those
is a migration, not a chrome change.

Ubuntu 24.04 native. No Docker/K8s required for the control plane.

## MVP success path

> Fresh Ubuntu 24.04 → install → Director admin → create hosting account →
> Linux tenant + Nginx site + PHP + DNS + TLS + MariaDB + SFTP + mailbox +
> backup/restore → Control self-serve → reboot healthy.

| Step | Status | Evidence |
| --- | --- | --- |
| Fresh Ubuntu 24.04 install | **PARTIAL** | `panel-install` phases + systemd units + `make package`. QEMU/rootfs scripts exist. Not proven in *this* agent VM (no Ubuntu host stack). |
| Director admin login | **VERIFIED** | Session + RBAC + Server Portal login form calling `/api/v1/auth/login`. Default admin is a **dev seed**, not a production secret. |
| Create hosting account | **VERIFIED** | `POST /accounts` → `account.provision` job; Director form and `panel-cli account create`. |
| Linux tenant | **VERIFIED** (live) / **MOCK** (sandbox) | `CreateLinuxUser` → `useradd` + `panel-sftp` + home harden when `live()`; otherwise home tree + `.panel-identity` only. |
| Nginx site | **VERIFIED** | `ApplyWebsite` writes `/etc/nginx/panel-sites/<id>.conf`, `nginx -t`, reload when live. Installer includes `panel-sites/*.conf`. |
| PHP 8.3 | **PARTIAL** | Pool file `/etc/php/8.3/fpm/pool.d/panel-<user>.conf` + nginx `fastcgi_pass` to `/run/php/panel-<user>.sock`. `ApplyWebsite` dropped `php_version`. `ensureFPM` swallowed start failures. |
| DNS | **PARTIAL** | Zone file + `named-zones.conf` + `pdns_control` when live. `ensureDomainStack` **ignored** `writeZone` errors unless ACME was live — account could go `active` with no published zone. |
| TLS | **PARTIAL** | HTTP-01 via `PANEL_ACME_DIRECTORY` / installer ACME phase; sandbox/dev issues `panel-dev` self-signed. Lab Pebble is optional. |
| MariaDB | **PARTIAL** | `CreateHostedDatabase` runs `mariadb -e` when live; **not** invoked from `account.provision`. Tenant must `POST /databases` or CLI. Sandbox is **MOCK** (`ObservedState=recorded`). |
| SFTP | **PARTIAL** | Live: `chpasswd`, `Match Group panel-sftp`, home `0751` + ACL. Virtual vsftpd users are a separate FTP path. No first-class SFTP probe in unit tests. |
| Mailbox | **PARTIAL** | Maps + Maildir + Dovecot passwd-file are real. Provision created `postmaster` with hash `!` (Dovecot cannot authenticate). Usable mailbox required a later API/CLI call. |
| Backup / restore | **VERIFIED** (local HPM1) / **PARTIAL** (offsite) | Worker `backup.create` / `backup.restore`; SFTP/S3 destinations need env. Restore must not unsuspend (tested). |
| Control self-serve | **VERIFIED** | Account Portal calls the same API with tenant capabilities; no host firewall/reboot chrome. |
| Reboot healthy | **PARTIAL** | `POST /server/reboot` + `confirm=REBOOT`; live `shutdown` only with `PANEL_ALLOW_REBOOT=1`. Installer enables units under `multi-user.target`. Full reboot loop is a host test, not CI. |

## Control plane (honest)

| Area | Status | Notes |
| --- | --- | --- |
| PostgreSQL desired state | **VERIFIED** | Migrations, seed package, jobs, audit. Memory store used in unit tests. |
| Auth / RBAC / audit | **VERIFIED** | Argon2id, capabilities, IDOR tests under `tests/security`. |
| Job engine | **VERIFIED** | Durable jobs, provision/reconcile/retire, suspend race tests. |
| Typed agent dispatch | **VERIFIED** | Allow-listed methods; path policy; no arbitrary root shell. |
| Installer phases | **PARTIAL** | Resumable; apt allow-list; writes stack files. Live apt/systemd needs a real 24.04 host. |
| Director / Control SPAs | **VERIFIED** | Real API forms (accounts, sites, DNS, mail, files, backups). Not placeholder screens. |
| WordPress manager | **PARTIAL** / out of MVP | `InstallWordPress` exists; **do not expand** this run. |
| Node / Python platform | **PARTIAL** / out of MVP | `ApplyAppUnit` stubs; **do not expand** this run. |
| Billing / WHMCS | **MISSING** | Explicit non-goal. |
| Windows / K8s control | **MISSING** | Explicit non-goal. |

## Highest-leverage slice (this run)

Close the **lying** provision steps on the MVP path without inventing
post-MVP product:

1. Brand surfaces: Director / Control / Agent / OpenAPI / docs / `kelmor-*`
   binary aliases. Keep Zone A/B/C paths and `panel-*` compatibility names.
2. `account.provision` must **fail** if DNS zone publish fails.
3. `account.provision` must create a default MariaDB (`<user>_db`) through
   the existing typed op (sandbox remains recorded-only).
4. `account.provision` must create a login mailbox `info@<primary>` hashed
   from the owner password — no `!` stub presented as a mailbox.
5. Pass PHP version through `ApplyWebsite`; surface php-fpm start errors
   when the agent is live.

## Remaining MVP blockers (after this slice)

- Prove the path on a **real Ubuntu 24.04** host (installer → Director →
  provision → HTTP/PHP/DNS/TLS/IMAP/SFTP/MariaDB → Control → reboot).
  This cloud workspace is not that host.
- Live ACME for customer hostnames (needs public DNS to the node).
- Offsite backup destinations configured and restored.
- Production admin password / TLS for Director:8443 and Control:8444.
- Optional: migrate on-disk `panel` paths to `kelmor` (separate, breaking).

## What this repo must not grow in an MVP run

WordPress productization, Node/Python platform, reseller billing, arbitrary
root shell, hardcoded `kelmor.host` as a customer hosting domain, Docker/K8s
as a required control-plane runtime.
