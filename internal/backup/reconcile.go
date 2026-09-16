package backup

import (
	"context"
	"fmt"

	"github.com/hosting-panel/panel/internal/store"
)

func PersistObjectKey(run *store.BackupRun, key string) error {
	if run == nil || key == "" {
		return fmt.Errorf("backup and object key are required")
	}
	run.State = "uploading"
	if run.Manifest == nil {
		run.Manifest = map[string]any{}
	}
	run.Manifest["key"] = key
	return nil
}

func ReconcileUploaded(ctx context.Context, repo Repository, run *store.BackupRun, man Manifest) error {
	if run == nil {
		return fmt.Errorf("backup is required")
	}
	key, _ := run.Manifest["key"].(string)
	if key == "" {
		return fmt.Errorf("backup object key missing")
	}
	sum := man.Checksums["object"]
	if sum == "" {
		sum = man.Checksums["files.tar.gz"]
	}
	if err := repo.Verify(ctx, key, sum); err != nil {
		return err
	}
	run.State = "succeeded"
	run.Checksum = sum
	if run.Manifest == nil {
		run.Manifest = map[string]any{}
	}
	run.Manifest["format_version"] = man.FormatVersion
	run.Manifest["key"] = key
	run.Manifest["checksums"] = man.Checksums
	run.Manifest["key_identity"] = man.KeyIdentity
	run.Manifest["databases"] = man.Databases
	run.Manifest["mailboxes"] = man.Mailboxes
	return nil
}
