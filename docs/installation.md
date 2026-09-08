# Installation

Target: Ubuntu 24.04 LTS, amd64 or arm64, no container runtime required for the control plane.

Minimum production: 4 vCPU, 8 GB RAM, 100 GB SSD.

```bash
sudo ./panel-install --hostname panel.example.net --admin-email ops@example.net --non-interactive --channel stable
```

Resume is automatic via `/var/lib/panel/install-state.json`. Structured logs are appended as JSON lines.

Development:

```bash
./panel-install --dev --non-interactive --hostname localhost
```

Phases (resumable): preflight, repositories, system packages (apt allow-list including vsftpd/libpam-pwdfile; verify `dpkg`), panel users, control database, control plane, web stack, database stack (start MariaDB/PostgreSQL when present), DNS (PowerDNS bind-files + loopback API), mail (Postfix virtual + Dovecot passwd-file), security (Fail2ban + SFTP chroot + vsftpd virtual users + ModSecurity/Rspamd/ClamAV files), firewall (`table inet panel`), runtime versions, templates, TLS (HTTP-01 webroot plus optional local Pebble CA when `/usr/local/panel/bin/pebble` is installed; the ACME account key is reused from `/var/lib/panel/secrets/acme-account.pem`; set `PANEL_ACME_DIRECTORY` to the Let’s Encrypt directory on a public host), systemd units, host runtime (start nginx/php-fpm/mail/PowerDNS with `--config-dir=/etc/powerdns` plus vsftpd and the control plane when systemd is blocked; health checks require HTTP, SMTP, DNS `:53`, and the API), portals (built Server/Account SPAs on `:8443` / `:8444` with `/api` proxied to the control API), administrator, health checks, installation report. A complete `dns` phase is re-applied if PowerDNS is not listening. The worker re-queues `certificate.provision` when `not_after` is within 30 days. Domain provision publishes the DNS zone first, then issues a certificate: HTTP-01 against `PANEL_ACME_DIRECTORY` or `/var/lib/panel/acme.directory` on a live agent, otherwise a local `panel-dev` certificate.

On Ubuntu 24.04 the installer creates the `panel` system user and `panel_control` database, grants that role CONNECT/table rights, copies `panel-*` binaries into `/usr/local/panel/bin`, and writes `/var/lib/panel/install-state.json`. A phase marked complete is re-applied when its verify check fails (for example a missing `panel` user or `/usr/local/panel/bin`).

`panel-api` and `panel-worker` refuse to start as root. Use `scripts/run-control-plane.sh panel-api` (and the same for `panel-worker`) so they run as `panel` and call the agent only over `/run/panel/agent.sock`.

Production `system_packages` apt-gets only allow-listed packages and requires root on Ubuntu 24.04. `--dev` writes the same files under `var/panel/host` without apt.
