# Upgrade

`panel-updater verify` checks an Ed25519-signed release manifest. Payloads that say `unsigned-development` are refused.

## Automatic releases

Every push to `main` runs `.github/workflows/release.yml`, which builds binaries,
portals, the Debian package, and a signed `stable` update feed, then uploads
artifacts. Push releases are artifact-only and never contact a VM, even when
publish secrets are configured.

Manual `workflow_dispatch` releases are also artifact-only by default. Select
**Publish feed** explicitly to copy the signed feed to the configured rsync
target or VM update directory (`/usr/local/panel/share/updates/`). An explicitly
requested publish fails when the target is unavailable or incomplete.

Local or CI one-shot:

```bash
make release          # build + sign feed only
make auto-update      # explicit build + sign + publish (uses .env or env vars)
bash scripts/ci/auto-update.sh
```

Version resolution (`scripts/ci/resolve-release-version.sh`):

1. `PANEL_UPDATE_RELEASE` when set explicitly
2. Exact git tag on `HEAD` (without the `v` prefix)
3. `platform_release` from `release-manifest.yaml` plus `GITHUB_RUN_NUMBER` or commit count

Required GitHub secret for signed CI releases:

| Secret | Purpose |
| --- | --- |
| `PANEL_UPDATE_SIGNING_KEY` | Ed25519 **private** key. Accepted formats: 128-char hex private key, 64-char hex seed, OpenSSH PEM (`-----BEGIN OPENSSH PRIVATE KEY-----`), or the PEM file hex-encoded. Must match `installer/phases/release.pub`. Do not paste `release.pub` itself. |
| `KELMOR_VM_HOST` | Optional VM used only by an explicit manual publish |
| `KELMOR_VM_USER` | SSH user (default `ubuntu`) |
| `KELMOR_VM_SSH_KEY` | Private SSH key for the VM |
| `KELMOR_VM_PORT` | Optional SSH port |
| `PANEL_UPDATE_PUBLISH_URL` | Optional manual rsync target instead of VM SSH |

Hosts poll the local nginx feed at `https://127.0.0.1:2087/updates/{channel}/manifest.json`,
served from `/usr/local/panel/share/updates/`. `get-kelmor.sh` / `make installer`
now ships that signed current-release feed so **Check now** can return idle
when the host is already on the installed release. A newer feed still requires
an explicit publish (`workflow_dispatch` + **Publish feed**, or the commands
below).

Hosts poll the feed via `panel-update.timer` or Director **Check now**.
Provision a new VM separately, download the signed workflow artifact, and upload
its `update-feed/` contents manually unless an operator intentionally runs the
manual publish path.

### Live host refresh (kelmor.host:2087)

Check now 404s when `/usr/local/panel/share/updates/stable/manifest.json` is
missing. nginx `/updates/` is the feed alias; do not confuse it with the SPA
routes `/updates/preferences` and `/updates/changelog`. This refresh does **not**
install a newer release or change automatic-install policy.

1. Build a signed feed (CI `kelmor-release-*` artifact, or locally):

   ```bash
   PANEL_UPDATE_SIGNING_KEY=/path/to/update-signing.priv make release
   ```

2. Install the feed on the host (root):

   ```bash
   sudo rsync -a --delete dist/update-feed/ /usr/local/panel/share/updates/
   sudo chown -R root:root /usr/local/panel/share/updates
   curl -kfsS https://127.0.0.1:2087/updates/stable/manifest.json >/dev/null
   ```

3. Refresh host binaries and Director chrome for this release (SPA + bins +
   updater). Do not run `panel-updater install` as part of the check fix:

   ```bash
   sudo install -m 0755 dist/bin/panel-api dist/bin/panel-agent dist/bin/panel-updater \
     dist/bin/panel-worker /usr/local/panel/bin/
   sudo rsync -a --delete dist/share/portals/server/ /usr/local/panel/share/portals/server/
   sudo systemctl restart panel-api panel-agent panel-worker
   sudo systemctl reload nginx
   ```

4. In Kelmor Director → Software Updates, click **Check now**. Expected: idle
   and up to date when the published feed release is not newer than
   `/usr/local/panel/current-release`, or available when it is. Last checked
   stamps only on success.

Alternatively, rerun `get-kelmor.sh` from a release built after this change;
that tarball includes `share/updates/stable/manifest.json` and `install.sh`
copies it into place.

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

Apply journals every runtime-asset target from the shared inventory (binaries, aliases, installer, units, timers, portals, templates, policy, and migrations). File rollback restores those runtime files and `current-release`. Forward-compatible schema migrations are not reverted: applied SQL and `share/migrations` stay in place so a failed activation can roll binaries back without undoing expand/backfill work. The operator then restarts `panel-api`, `panel-worker`, and `panel-agent`.

The updater never runs `apt-get dist-upgrade` or other unattended OS upgrades.
