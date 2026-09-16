# Encrypted backups

New backups use HPM3: an [age](https://age-encryption.org/) STREAM envelope (ChaCha20-Poly1305 frames) around authenticated metadata, a declared consistency fence, an expected component inventory, and streamed home/database/mailbox parts. Per-backup file keys are wrapped to a control-plane identity derived from the master key. The ciphertext object hash is verified after upload. Existing HPM1 backups remain readable: format 1 is home-only and format 2 packs `home.tar.gz`, `databases/<engine>/<name>.sql`, and `mail/<domain>/<local>.tar.gz`. Checksums and the object key are recorded on the job and in `backup_runs.manifest`.

The worker never shells out as root. The typed agent dumps MariaDB/PostgreSQL and packs mailbox trees. A backup succeeds only when every expected home, database, and mailbox component is captured exactly once. The object key is persisted before upload so a crash after PUT can still reconcile the stored object. Restore pins actor, account, object, manifest, format, and key identity in an account-unique journal, holds an account write fence, and resumes from the last checkpoint. A failed rollback keeps maintenance and `manual_intervention`. Archive entries that escape the destination are rejected.

Restore is in-place onto the same Linux username. Cross-username and cross-account substitution are refused. A second restore for an account that already has an active journal is rejected.

SFTP and S3 repositories implement the same `Repository` interface. Destination `sftp` uses `golang.org/x/crypto/ssh` plus `github.com/pkg/sftp` when `PANEL_SFTP_HOST` is set:

- `PANEL_SFTP_HOST` — `host` or `host:port` (default port 22)
- `PANEL_SFTP_USER` — remote user (default `panel-backup`)
- `PANEL_SFTP_PASSWORD` and/or `PANEL_SFTP_KEY` / `PANEL_SFTP_KEY_PEM`
- `PANEL_SFTP_HOST_KEY` — required SHA256 host-key fingerprint (`SHA256:…`)
- `PANEL_SFTP_ROOT` — remote directory (default `/var/lib/panel/offsite`)

Without `PANEL_SFTP_HOST`, objects are written atomically under `PANEL_SFTP_ROOT` on this node. The installer creates the `panel-backup` system user, an ed25519 client key, and `/var/lib/panel/secrets/backup-sftp.env` so destination `sftp` pushes over SSH to `127.0.0.1` (every sshd host-key type pinned). Objects land in `/var/lib/panel/offsite/inbox` so the account home can stay `0750` for sshd StrictModes. Override those variables for a remote receiver.

Destination `s3` uses SigV4 against `PANEL_S3_ENDPOINT` / `PANEL_S3_BUCKET`. The installer starts `panel-object-store` on `127.0.0.1:19090` (path-style, objects under `/var/lib/panel/objects`) and writes `/var/lib/panel/secrets/backup-s3.env`. Point those variables at AWS or MinIO for a remote bucket. Local, SFTP, and S3 repositories stream PUT/GET and reject a non-empty object whose checksum does not match. The worker launcher and `panel-worker.service` load `backup-sftp.env` and `backup-s3.env`.
