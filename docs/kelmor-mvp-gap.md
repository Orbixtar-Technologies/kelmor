# Kelmor MVP gap (PR #3 QEMU TLS / reboot / Control @ this tree)

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
| Fresh Ubuntu 24.04 install | **VERIFIED** (nested QEMU) | Official Noble cloud image, systemd PID 1, `panel-install --acme pebble`. Not a bare-metal ISO. First boot left a competing `pdns_server` daemon; this slice prefers `systemctl restart pdns`. |
| Director admin login | **VERIFIED** | API login + Kelmor Director HTML on guest `:8443` (`director-ok`). Default admin is a **dev seed**. |
| Create hosting account | **VERIFIED** | `POST /accounts` → `account.provision`; guest account `freshhost` became `active`. |
| Linux tenant | **VERIFIED** (live) / **MOCK** (sandbox) | Guest `freshhost` UID 20000, home `0751 root:root`, shell `nologin`. |
| Nginx site | **VERIFIED** | Guest `Host: freshhost.test` HTTP 200. |
| PHP 8.3 | **VERIFIED** (live QEMU) | `GET /index.php` → `php 8.3.6 freshhost`; pool isolated; `/run/php/panel-freshhost.sock`; `php8.3-fpm` active. |
| DNS | **VERIFIED** (after systemd pdns) | Zone file + `dig @127.0.0.1 freshhost.test A` → `10.0.2.15`. First installer pass answered empty until `systemctl restart pdns` (daemon vs unit). |
| TLS | **VERIFIED** (Pebble HTTP-01 on nested QEMU) / **PARTIAL** (public LE) | Guest `freshhost.test` leaf issued by `CN=Pebble Intermediate CA 7e0d29`, HTTPS 200 via `--resolve`. Worker persists the ACME account key under panel-owned `PANEL_STATE_DIR/control/` (Zone A). Public Let’s Encrypt still needs a routable A/AAAA + :80. |
| MariaDB | **VERIFIED** (live) / **MOCK** (sandbox) | Provision created `freshhost_db`; `SHOW DATABASES` + credential file on the guest. |
| SFTP | **VERIFIED** (live QEMU) | Match-scoped password auth; `sftp` as `freshhost` listed `public_html`. |
| Mailbox | **VERIFIED** | `doveadm auth test info@freshhost.test` with the owner password. After reboot, `dovecot.service` was `active` (no leftover hand-started master). |
| Backup / restore | **VERIFIED** (local HPM1) / **PARTIAL** (offsite) | Guest `backup.create` local succeeded. Offsite not run. |
| Control self-serve | **VERIFIED** (Chrome) | `CONTROL_UI_MVP_OK freshhost control@freshhost.test` — Kelmor Control login, create mailbox `control`, SSL page lists `freshhost.test`, API agrees. |
| Reboot healthy | **VERIFIED** (nested QEMU) | `KELMOR_REBOOT_HEALTH_OK` after `shutdown -r now` with no `systemctl start` repair: panel-* + nginx/php-fpm/postgresql/postfix/dovecot/pdns/mariadb/pebble, tenant HTTP/PHP/DNS/MariaDB/SFTP/mail/HTTPS. `systemctl is-system-running` was **degraded** only because `quotaon.service` failed (cloud image has no usrquota). |

## Control plane (honest)

| Area | Status | Notes |
| --- | --- | --- |
| PostgreSQL desired state | **VERIFIED** | Migrations, seed package, jobs, audit. Memory store used in unit tests. |
| Auth / RBAC / audit | **VERIFIED** | Argon2id, capabilities, IDOR tests under `tests/security`. |
| Job engine | **VERIFIED** | Durable jobs, provision/reconcile/retire, suspend race tests. |
| Typed agent dispatch | **VERIFIED** | Allow-listed methods; path policy; no arbitrary root shell. |
| Installer phases | **VERIFIED** (QEMU guest) / **PARTIAL** (host-runtime vs systemd) | Live apt + units on nested Noble. Prefer systemd for pdns/dovecot so a daemon does not steal the port. |
| Director / Control SPAs | **VERIFIED** | Real API forms (accounts, sites, DNS, mail, files, backups). Not placeholder screens. |
| WordPress manager | **PARTIAL** / out of MVP | `InstallWordPress` exists; **do not expand** this run. |
| Node / Python platform | **PARTIAL** / out of MVP | `ApplyAppUnit` stubs; **do not expand** this run. |
| Billing / WHMCS | **MISSING** | Explicit non-goal. |
| Windows / K8s control | **MISSING** | Explicit non-goal. |

## Highest-leverage slice (PR #1, now on main)

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

## Proof harness (this slice)

Focused path (no WordPress / Node / Python / cPanel import):

```bash
# host tools: qemu-system-x86, OVMF, cloud-localds
make qemu-host-ready
make build
# optional: make portals   # required for Director HTML on :8443
sudo ./scripts/qemu-kelmor-path.sh
```

`qemu-kelmor-path.sh` boots `scripts/qemu-fresh-guest.sh` (official Noble
cloud image + `panel-install --acme pebble`) then:

1. `scripts/qemu-kelmor-mvp.sh` → `fresh-provision-smoke.sh` (privilege
   zone A, Director HTML, PHP, DNS, **customer TLS**, MariaDB, `info@`,
   password SFTP).
2. `scripts/qemu-kelmor-reboot.sh` → guest `shutdown -r now` →
   `guest-reboot-health.sh` (no `systemctl start` repair).
3. `scripts/control-ui-mvp.sh` → Chrome against Control `:8444` (host
   forward `39444`) as the provisioned tenant.

`scripts/qemu-mvp.sh` still runs the broader `live-e2e.sh` (import/WP).
Do not treat that as the Kelmor MVP gate.

### Customer TLS modes (honest)

| Mode | Install flag / env | What it proves | What it does **not** prove |
| --- | --- | --- | --- |
| `panel-dev` | empty `PANEL_ACME_DIRECTORY` (dev/sandbox) | Self-signed Kelmor leaf written through `IssueDevCertificate` | Public trust, HTTP-01 |
| Pebble (lab) | `--acme pebble` | Real ACME HTTP-01 against a local CA; same challenge protocol as LE | Public hostname, public CA, inbound :80 from the Internet |
| Let’s Encrypt **staging** | `--acme staging` or `PANEL_ACME_STAGING=1` | Public CA staging issuance | Production trust store (staging intermediates are untrusted by browsers) |
| Let’s Encrypt production | `--acme letsencrypt` (default on a fresh public install) | Public trust | Nothing if DNS/HTTP-01 cannot reach the node |

Public hostnames still require **all** of:

1. An A/AAAA at the registrar (or parent) pointing at the node’s
   `PANEL_PUBLIC_IPV4` (not a QEMU user-net `10.0.2.15` that the Internet
   cannot route to).
2. Port 80 reachable from Let’s Encrypt validators (nftables `table inet
   panel` already allows HTTP).
3. Install with `--acme staging` first, then `--acme letsencrypt` once
   staging HTTP-01 succeeds — do not burn production rate limits on a
   broken path.
4. Nested QEMU user networking **cannot** satisfy (1)+(2). That is why
   the QEMU proof uses Pebble, not LE staging.

## Remaining MVP blockers (after this slice)

- Let’s Encrypt **staging/production** on a host with public DNS to :80
  (QEMU user-net cannot satisfy this; see table above).
- Nested KVM on some hosts hits `kvm_spurious_fault`; this proof used TCG
  (`PANEL_QEMU_ACCEL=tcg`). Prefer KVM only when dmesg is clean.
- Offsite backup destinations configured and restored.
- Production admin password / TLS for Director:8443 and Control:8444.
- `quotaon.service` on images without usrquota leaves systemd `degraded`
  (MVP services were still active).
- Optional: migrate on-disk `panel` paths to `kelmor` (separate, breaking).

## What this repo must not grow in an MVP run

WordPress productization, Node/Python platform, reseller billing, arbitrary
root shell, hardcoded `kelmor.host` as a customer hosting domain, Docker/K8s
as a required control-plane runtime.
