package operations

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBandwidthCheckpointSkipsRereadAndRotation(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	id := "sitebw2"
	if err := os.MkdirAll(filepath.Join(root, "var/log/nginx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc/nginx/panel-sites"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := "server {\n    root /home/bwuser/public_html;\n    server_name bw.test;\n}\n"
	if err := os.WriteFile(filepath.Join(root, "etc/nginx/panel-sites", id+".conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	logPath := filepath.Join(root, "var/log/nginx", id+".access.log")
	line := now.Format(time.RFC3339) + " 100\n"
	if err := os.WriteFile(logPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	ids := h.websiteIDsForAccount("bwuser")
	first := h.sumNginxBandwidth("bwuser", ids, now)
	if first != 100 {
		t.Fatalf("first=%d", first)
	}
	h.persistBandwidthTotal("bwuser", now, first)
	second := h.sumNginxBandwidth("bwuser", ids, now)
	if second != 100 {
		t.Fatalf("reread doubled: %d", second)
	}
	if err := os.Rename(logPath, logPath+".1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte(now.Format(time.RFC3339)+" 25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rotated := h.sumNginxBandwidth("bwuser", ids, now)
	if rotated != 125 {
		t.Fatalf("rotation=%d", rotated)
	}
	h.persistBandwidthTotal("bwuser", now, rotated)
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	truncated := h.sumNginxBandwidth("bwuser", ids, now)
	if truncated != 125 {
		t.Fatalf("truncate re-counted: %d", truncated)
	}
	nextMonth := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if err := os.WriteFile(logPath, []byte(nextMonth.Format(time.RFC3339)+" 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := h.sumNginxBandwidth("bwuser", ids, nextMonth)
	if fresh != 7 {
		t.Fatalf("month boundary=%d", fresh)
	}
}
