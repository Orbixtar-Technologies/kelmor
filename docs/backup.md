# Encrypted backups

Account backups are HPM1 bundles: an AES-256-GCM envelope (control-plane master key) around a JSON manifest and a `files.tar.gz` of the account home. Checksums are recorded on the job and the object key lives in `backup_runs.manifest`.

The worker never shells out as root. It reads the sandboxed or live home, writes to `/var/lib/panel/backups` (or `PANEL_STATE_DIR/host/var/lib/panel/backups` in development), and restores only after magic, format, checksum, and username preflight succeed. Archive entries that escape the destination are rejected.

Restore is in-place onto the same Linux username. Cross-username restore is refused.

SFTP and S3 repositories implement the same `Repository` interface; this tree ships the local adapter. Encrypted transport for off-host copies is a follow-on installer phase, not a different backup format.
