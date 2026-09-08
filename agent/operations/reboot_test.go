package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRebootHostRecordsWithoutShutdown(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	res, err := h.rebootHost()
	if err != nil {
		t.Fatal(err)
	}
	if res.ObservedState != "reboot-scheduled" {
		t.Fatalf("%+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/reboot-requested")); err != nil {
		t.Fatal(err)
	}
}
