package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStatusUsesPanelReadableMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-status.json")
	if err := WriteStatus(path, Status{
		State: "idle", InstalledRelease: "0.1.0",
		Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestReconcileStatusPermissionsFixesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-status.json")
	if err := os.WriteFile(path, []byte(`{"state":"idle"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileStatusPermissions(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
