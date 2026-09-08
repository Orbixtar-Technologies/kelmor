# Upgrade

`panel-updater verify` checks an Ed25519-signed release manifest. Payloads that say `unsigned-development` are refused.

```bash
panel-updater verify ./bundle/manifest.json ./release.pub
panel-updater apply ./bundle /usr/local/panel ./release.pub
# on failed health-check:
panel-updater rollback /usr/local/panel
```

Apply snapshots `/usr/local/panel/bin` into `rollback/`, copies hashed artifacts, and writes `current-release`. Rollback restores the snapshot. The operator then restarts `panel-api`, `panel-worker`, and `panel-agent`.

The updater never runs `apt-get dist-upgrade` or other unattended OS upgrades.
