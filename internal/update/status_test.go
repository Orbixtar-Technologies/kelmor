package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStatusReplacesStatusAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-status.json")
	if err := os.WriteFile(path, []byte(`{"state":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	want := Status{
		State: "available", InstalledRelease: "1.0.0",
		AvailableRelease: "2.0.0", LastCheckedAt: "2026-09-09T15:00:00Z",
		Automatic: true, Channel: "stable",
	}
	if err := WriteStatus(path, want); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Status
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("status mismatch: got %+v want %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("status mode = %o, want 600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "update-status.json" {
		t.Fatalf("temporary status file left behind: %v", entries)
	}
}
