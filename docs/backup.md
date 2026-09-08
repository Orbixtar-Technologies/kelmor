# Encrypted backups

Account backups are HPM1 bundles: an AES-256-GCM envelope (control-plane master key) around a JSON manifest and a payload. Format 2 packs `home.tar.gz`, `databases/<engine>/<name>.sql`, and `mail/<domain>/<local>.tar.gz`. Format 1 (home-only) still restores. Checksums are recorded on the job and the object key lives in `backup_runs.manifest`.

The worker never shells out as root. The typed agent dumps MariaDB/PostgreSQL and packs mailbox trees, then the worker seals the encrypted object under `/var/lib/panel/backups`. Restore unpacks the home, imports each SQL dump, and replaces `/var/vmail/<domain>/<local>` before rewriting mail maps. Archive entries that escape the destination are rejected.

Restore is in-place onto the same Linux username. Cross-username restore is refused.

SFTP and S3 repositories implement the same `Repository` interface. Destination `sftp` uses `golang.org/x/crypto/ssh` plus `github.com/pkg/sftp` when `PANEL_SFTP_HOST` is set:

- `PANEL_SFTP_HOST` — `host` or `host:port` (default port 22)
- `PANEL_SFTP_USER` — remote user (default `panel-backup`)
- `PANEL_SFTP_PASSWORD` and/or `PANEL_SFTP_KEY` / `PANEL_SFTP_KEY_PEM`
- `PANEL_SFTP_HOST_KEY` — required SHA256 host-key fingerprint (`SHA256:…`)
- `PANEL_SFTP_ROOT` — remote directory (default `/var/lib/panel/offsite`)

Without `PANEL_SFTP_HOST`, objects are written atomically under `PANEL_SFTP_ROOT` on this node. The installer creates the `panel-backup` system user, an ed25519 client key, and `/var/lib/panel/secrets/backup-sftp.env` so destination `sftp` pushes over SSH to `127.0.0.1` (every sshd host-key type pinned). Objects land in `/var/lib/panel/offsite/inbox` so the account home can stay `0750` for sshd StrictModes. Override those variables for a remote receiver.

Destination `s3` uses SigV4 against `PANEL_S3_ENDPOINT` / `PANEL_S3_BUCKET`. The installer starts `panel-object-store` on `127.0.0.1:19090` (path-style, objects under `/var/lib/panel/objects`) and writes `/var/lib/panel/secrets/backup-s3.env`. Point those variables at AWS or MinIO for a remote bucket. The HPM1 envelope is unchanged. The worker launcher and `panel-worker.service` load `backup-sftp.env` and `backup-s3.env`.
