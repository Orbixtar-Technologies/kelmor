package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStampInstalledReleaseRewritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-status.json")
	if err := WriteStatus(path, Status{
		State: "idle", InstalledRelease: "0.2.312", Automatic: true, Channel: "stable",
	}); err != nil {
		t.Fatal(err)
	}
	if err := StampInstalledRelease(path, "0.2.325"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(string(raw), `"installed_release":"0.2.325"`) || containsAll(string(raw), "0.2.312") {
		t.Fatalf("status %s", raw)
	}
}

func containsAll(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

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
