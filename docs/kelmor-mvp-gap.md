# Kelmor MVP gap (PR #2 QEMU path @ this tree)

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
| TLS | **PARTIAL** | Director `:8443` self-signed worked. Customer hostname ACME still lab/Pebble/`panel-dev`, not public DNS. |
| MariaDB | **VERIFIED** (live) / **MOCK** (sandbox) | Provision created `freshhost_db`; `SHOW DATABASES` + credential file on the guest. |
| SFTP | **VERIFIED** (live QEMU) | Match-scoped password auth; `sftp` as `freshhost` listed `public_html`. |
| Mailbox | **VERIFIED** (auth) / **PARTIAL** (unit) | `doveadm auth test info@freshhost.test` succeeded with the owner password. Dovecot systemd unit was `failed` while a hand-started master still served IMAP. |
| Backup / restore | **VERIFIED** (local HPM1) / **PARTIAL** (offsite) | Guest `backup.create` local succeeded. Offsite not run. |
| Control self-serve | **VERIFIED** (API/SPA) | Same API as Director; this run did not click Control in a browser. |
| Reboot healthy | **PARTIAL** | Guest was not rebooted. |

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
cloud image + `panel-install`) then `scripts/qemu-kelmor-mvp.sh`, which
runs `scripts/fresh-provision-smoke.sh` inside the guest. That smoke now
asserts privilege zone A (API not root), Kelmor Director HTML when
`:8443` is required, PHP execution + fpm socket, published PowerDNS A,
default MariaDB `<user>_db`, `info@` with the owner password, and a
password SFTP listing.

`scripts/qemu-mvp.sh` still runs the broader `live-e2e.sh` (import/WP).
Do not treat that as the Kelmor MVP gate.

## Remaining MVP blockers (after this slice)

- Re-run `qemu-kelmor-path.sh` on a **new** empty disk after the
  systemd-pdns fix (this run repaired pdns on an already-installed guest).
- Guest reboot → units come back healthy (`PANEL_ALLOW_REBOOT=1`).
- Dovecot managed only by systemd (no leftover master.pid clash).
- Live ACME for customer hostnames (public DNS to the node).
- Offsite backup destinations configured and restored.
- Production admin password / TLS for Director:8443 and Control:8444.
- Browser pass of Kelmor Control (API path already used).
- Optional: migrate on-disk `panel` paths to `kelmor` (separate, breaking).

## What this repo must not grow in an MVP run

WordPress productization, Node/Python platform, reseller billing, arbitrary
root shell, hardcoded `kelmor.host` as a customer hosting domain, Docker/K8s
as a required control-plane runtime.
