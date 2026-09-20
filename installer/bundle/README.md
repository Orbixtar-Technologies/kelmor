# Kelmor installer

Self-contained Ubuntu 24.04 host installer. Prefer the GitHub one-liner:

```bash
curl -fsSL https://github.com/Orbixtar-Technologies/kelmor/releases/latest/download/get-kelmor.sh | sudo bash -s -- --admin-password 'your-password'
```

This tarball does not contact the lab VM. It includes a signed
`share/updates/stable/manifest.json` when the installer was built with
`PANEL_UPDATE_SIGNING_KEY`, so Director **Check now** can read the local nginx
feed after `install.sh` copies that tree to `/usr/local/panel/share/updates/`.

Offline install:

```bash
tar -xzf kelmor-installer_*_linux_*.tar.gz
cd kelmor-installer_*
sudo ./install.sh --hostname panel.example.net --admin-email ops@example.net --non-interactive
```

Or copy `install.yaml.example` to `install.yaml` and run `sudo ./install.sh`.
Without `--non-interactive`, `panel-install` prompts for hostname, admin
email, and ACME directory on a TTY.

Resume is stored at `/var/lib/panel/install-state.json`.
