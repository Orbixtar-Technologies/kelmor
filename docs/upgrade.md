# Upgrade

`panel-updater verify` checks an Ed25519-signed release manifest. Payloads that say `unsigned-development` are refused.

## Automatic releases

Every push to `main` runs `.github/workflows/release.yml`, which builds binaries,
portals, the Debian package, and a signed `stable` update feed, then uploads
artifacts. When publish secrets are configured, the feed is copied to the host
update directory (`/usr/local/panel/share/updates/`).

Local or CI one-shot:

```bash
make release          # build + sign feed only
make auto-update      # build + sign + publish (uses .env or env vars)
bash scripts/ci/auto-update.sh
```

Version resolution (`scripts/ci/resolve-release-version.sh`):

1. `PANEL_UPDATE_RELEASE` when set explicitly
2. Exact git tag on `HEAD` (without the `v` prefix)
3. `platform_release` from `release-manifest.yaml` plus `GITHUB_RUN_NUMBER` or commit count

Required GitHub secrets for signed CI releases:

| Secret | Purpose |
| --- | --- |
| `PANEL_UPDATE_SIGNING_KEY` | Ed25519 **private** key as hex (128 chars, or 64-char seed). Must match `installer/phases/release.pub`. Do not paste `release.pub` itself. |
| `KELMOR_VM_HOST` | Optional VM to publish the feed |
| `KELMOR_VM_USER` | SSH user (default `ubuntu`) |
| `KELMOR_VM_SSH_KEY` | Private SSH key for the VM |
| `KELMOR_VM_PORT` | Optional SSH port |
| `PANEL_UPDATE_PUBLISH_URL` | Optional rsync target instead of VM SSH |

Hosts poll the feed via `panel-update.timer` or Director **Check for updates**.

Generate or rotate a signing keypair locally:

```bash
bash scripts/ci/generate-release-signing-key.sh .run/validation/update-signing.priv --install-pub
```

Copy the private key file contents into the `PANEL_UPDATE_SIGNING_KEY` GitHub secret.
After rotating `release.pub`, redeploy `/etc/panel/update.pub` on existing hosts.

```bash
panel-updater sign ./bundle 1.2.0 stable
panel-updater verify ./bundle/manifest.json ./bundle/release.pub
panel-updater apply ./bundle /usr/local/panel ./bundle/release.pub
# on failed health-check:
panel-updater rollback /usr/local/panel
```

Apply snapshots `/usr/local/panel/bin` into `rollback/`, copies hashed artifacts, and writes `current-release`. Rollback restores the snapshot. The operator then restarts `panel-api`, `panel-worker`, and `panel-agent`.

The updater never runs `apt-get dist-upgrade` or other unattended OS upgrades.
