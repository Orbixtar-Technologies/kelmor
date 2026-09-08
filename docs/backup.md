# Encrypted backups

Account backups are HPM1 bundles: an AES-256-GCM envelope (control-plane master key) around a JSON manifest and a payload. Format 2 packs `home.tar.gz`, `databases/<engine>/<name>.sql`, and `mail/<domain>/<local>.tar.gz`. Format 1 (home-only) still restores. Checksums are recorded on the job and the object key lives in `backup_runs.manifest`.

The worker never shells out as root. The typed agent dumps MariaDB/PostgreSQL and packs mailbox trees, then the worker seals the encrypted object under `/var/lib/panel/backups`. Restore unpacks the home, imports each SQL dump, and replaces `/var/vmail/<domain>/<local>` before rewriting mail maps. Archive entries that escape the destination are rejected.

Restore is in-place onto the same Linux username. Cross-username restore is refused.

SFTP and S3 repositories implement the same `Repository` interface; this tree ships the local adapter. Encrypted transport for off-host copies is a follow-on installer phase, not a different backup format.
