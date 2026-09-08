package operations

import "testing"

func TestRunFixedRejects(t *testing.T) {
	if _, err := runFixed("/bin/sh", "-c", "id"); err == nil {
		t.Fatal("shell must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "acme;id"); err == nil {
		t.Fatal("metachar must be rejected")
	}
}
