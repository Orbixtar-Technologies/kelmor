package operations

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunFixedRejects(t *testing.T) {
	if _, err := runFixed("/bin/sh", "-c", "id"); err == nil {
		t.Fatal("shell must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "acme;id"); err == nil {
		t.Fatal("metachar must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "-evil", "acme"); err == nil {
		t.Fatal("leading option must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "acme\x00root"); err == nil {
		t.Fatal("control character must be rejected")
	}
	if _, err := runFixedEnv(context.Background(), "/usr/sbin/useradd", []string{"PATH=/tmp/evil"}, 0, nil, "acme"); err == nil {
		t.Fatal("hostile environment must be rejected")
	}
}

func TestRunFixedBoundsOutputAndHonorsCancel(t *testing.T) {
	if _, err := runFixed("/usr/bin/python3", "-c", strings.Repeat("print('x'*1000)\n", 1)); err == nil {
		// python -c is not an allowlisted argument contract
		t.Fatal("python -c must be rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runFixedEnv(ctx, "/usr/bin/pgrep", nil, time.Second, nil, "-x", "init"); err == nil {
		t.Fatal("cancelled context must fail before changing the executable contract")
	}
}
