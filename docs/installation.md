# Installation

Target: Ubuntu 24.04 LTS, amd64 or arm64, no container runtime required for the control plane.

Minimum production: 4 vCPU, 8 GB RAM, 100 GB SSD.

```bash
# from this tree:
make package   # dist/deb/hosting-panel_*.deb (binaries, portals, systemd units)
sudo dpkg -i dist/deb/hosting-panel_*.deb
# optional: source .run/validation so hostname + public IPv4 + SMTP relay apply
# source scripts/load-validation-env.sh
sudo panel-install --hostname panel.example.net --admin-email ops@example.net --non-interactive --channel stable
# omit --hostname to use TEST_DOMAIN from .run/validation/domain.env
# Live installs refuse placeholder portal HTML. `make package` ships the built SPAs;
# a binary-only copy must place them at /usr/local/panel/share/portals/{server,account}.
# default ACME is Let's Encrypt HTTP-01. Lab/Pebble:
# sudo panel-install --acme pebble --non-interactive
```

Resume is automatic via `/var/lib/panel/install-state.json`. Structured logs are appended as JSON lines.

Development:

```bash
./panel-install --dev --non-interactive --hostname localhost
```

Phases (resumable): preflight, repositories, system packages (apt allow-list including vsftpd/libpam-pwdfile; verify `dpkg`), panel users, control database, control plane, web stack, database stack (start MariaDB/PostgreSQL when present), DNS (PowerDNS bind-files + loopback API), mail (Postfix virtual + Dovecot passwd-file), security (Fail2ban + SFTP chroot + vsftpd virtual users + ModSecurity/Rspamd/ClamAV files + loopback S3 object store on `:19090`), firewall (`table inet panel`), runtime versions, templates, TLS (HTTP-01 webroot on `:80`; a fresh production install writes `https://acme-v02.api.letsencrypt.org/directory` to `/var/lib/panel/acme.directory` and `acme.env`. `--acme pebble` or an existing Pebble directory keeps the local lab CA. `--acme staging` uses Let’s Encrypt staging. The ACME account key is reused from `/var/lib/panel/secrets/acme-account.pem`. `--root /path` prefixes packaged file writes for a chroot or image), systemd units (enabled under `multi-user.target.wants` so a real systemd boot starts the control plane), host runtime (start MariaDB, PostgreSQL, nginx/php-fpm/mail/PowerDNS with `--config-dir=/etc/powerdns`, vsftpd, the selected ACME listener, and the control plane when systemd is blocked; health checks require HTTP, SMTP, DNS `:53`, MariaDB/PostgreSQL when those binaries exist, and the API), portals (built Server/Account SPAs on HTTPS `:8443` / `:8444` with `/api` proxied to the control API; a self-signed SAN cert covers loopback, and the installer best-effort issues HTTP-01 for the panel hostname), administrator, health checks, installation report. A complete `dns` phase is re-applied if PowerDNS is not listening. The worker re-queues `certificate.provision` when `not_after` is within 30 days, and `certificate.portal` when the panel hostname certificate under `/var/lib/panel/certs/<hostname>.crt` is missing or inside that window. Domain provision publishes the DNS zone first, then issues a certificate: HTTP-01 against `PANEL_ACME_DIRECTORY` or `/var/lib/panel/acme.directory` on a live agent, otherwise a local `panel-dev` certificate.

On Ubuntu 24.04 the installer creates the `panel` system user and `panel_control` database, grants that role CONNECT/table rights, copies `panel-*` binaries into `/usr/local/panel/bin`, and writes `/var/lib/panel/install-state.json`. A phase marked complete is re-applied when its verify check fails (for example a missing `panel` user or `/usr/local/panel/bin`).

`panel-api` and `panel-worker` refuse to start as root. Use `scripts/run-control-plane.sh panel-api` (and the same for `panel-worker`) so they run as `panel` and call the agent only over `/run/panel/agent.sock`.

Production `system_packages` apt-gets only allow-listed packages and requires root on Ubuntu 24.04. `--dev` writes the same files under `var/panel/host` without apt.
