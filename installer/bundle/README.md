# Kelmor installer

Self-contained Ubuntu 24.04 host installer. Prefer the GitHub one-liner:

```bash
curl -fsSL https://github.com/OrbixtarTechnologies/public/releases/latest/download/get-kelmor.sh | sudo bash
```

This tarball does not contact the lab VM. Offline install:

```bash
tar -xzf kelmor-installer_*_linux_*.tar.gz
cd kelmor-installer_*
sudo ./install.sh --hostname panel.example.net --admin-email ops@example.net --non-interactive
```

Or copy `install.yaml.example` to `install.yaml` and run `sudo ./install.sh`.
Without `--non-interactive`, `panel-install` prompts for hostname, admin
email, and ACME directory on a TTY.

Resume is stored at `/var/lib/panel/install-state.json`.
