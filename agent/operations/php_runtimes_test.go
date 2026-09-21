package operations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListPHPRuntimesMarksOnlyHostBinariesInstalled(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "usr/sbin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "usr/sbin/php-fpm8.3"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}

	raw, err := h.Dispatch(context.Background(), Request{Method: "ListPHPRuntimes"})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := raw.([]map[string]any)
	if len(items) != 3 {
		t.Fatalf("catalog %v", items)
	}
	status := map[string]string{}
	for _, item := range items {
		status[item["version"].(string)] = item["status"].(string)
	}
	if status["8.3"] != "installed" {
		t.Fatalf("planted 8.3 should be installed: %v", items)
	}
	if status["8.4"] != "available" {
		t.Fatalf("missing 8.4 should stay available, not installed: %v", items)
	}
	if status["8.5"] != "available" {
		t.Fatalf("missing 8.5 should stay available, not installed: %v", items)
	}
}
