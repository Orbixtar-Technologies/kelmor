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
	if err := h.applyCgroupLimits("acme42", 50, 64<<20, 20); err != nil {
		t.Fatal(err)
	}
}
