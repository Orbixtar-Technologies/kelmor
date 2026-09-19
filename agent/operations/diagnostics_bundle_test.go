package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectDiagnosticsIncludesAllowlistedHostFiles(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if err := os.MkdirAll(filepath.Join(root, "etc/panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/panel/server-profile"), []byte("mail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/panel/cluster.json"), []byte(`{"peers":["https://peer.test"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := h.collectDiagnostics()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, file := range bundle.Files {
		found[file.Name] = file.Content
	}
	if !strings.Contains(found["etc-panel/server-profile"], "mail") {
		t.Fatalf("profile missing: %#v", found)
	}
	if !strings.Contains(found["etc-panel/cluster.json"], "peer.test") {
		t.Fatalf("cluster missing: %#v", found)
	}
	if found["host/system.txt"] == "" {
		t.Fatal("expected system summary")
	}
}

func TestRedactDiagnosticHidesSecrets(t *testing.T) {
	got := redactDiagnostic("ok\npassword=hunter2\ntoken=abc\nsafe\n")
	if strings.Contains(got, "hunter2") || strings.Contains(got, "abc") {
		t.Fatalf("secret leaked: %q", got)
	}
	if !strings.Contains(got, "safe") {
		t.Fatalf("safe line dropped: %q", got)
	}
}
