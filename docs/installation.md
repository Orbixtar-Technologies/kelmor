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

Phases (resumable): preflight, repositories, system packages, panel users, control database, control plane, web stack, database stack, DNS (PowerDNS bind-files + loopback API), mail (Postfix virtual + Dovecot passwd-file), security (Fail2ban + SFTP chroot + ModSecurity/Rspamd/ClamAV files), firewall (`table inet panel`), runtime versions, templates, TLS (HTTP-01 webroot), systemd units, host runtime (start nginx/php-fpm/mail when systemd is blocked), administrator, health checks, installation report.

On Ubuntu 24.04 the installer creates the `panel` system user and `panel_control` database, copies `dist/bin` into `/usr/local/panel/bin`, and writes `/var/lib/panel/install-state.json` so a failed phase can be re-run without repeating completed work.

Production `system_packages` apt-gets only allow-listed packages and requires root on Ubuntu 24.04. `--dev` writes the same files under `var/panel/host` without apt.
