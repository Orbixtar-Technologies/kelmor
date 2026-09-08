package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAttachPIDWritesProcs(t *testing.T) {
	dir := t.TempDir()
	attachPID(dir, 4242)
	b, err := os.ReadFile(filepath.Join(dir, "cgroup.procs"))
	if err != nil || string(b) != "4242\n" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestApplyCgroupLimitsSkippedWhenNotLive(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := h.applyCgroupLimits("acme42", 50, 64<<20, 20, 80, 250); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(h.Root, "var/lib/panel/cgroup/acme42"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsCgroup(string(b), "io_weight=80") || !containsCgroup(string(b), "iops=250") {
		t.Fatal(string(b))
	}
}

func containsCgroup(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexCgroup(s, sub) >= 0)
}

func indexCgroup(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
