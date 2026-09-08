package phases

import "testing"

func TestPathMountedRoot(t *testing.T) {
	if !pathMounted("/") && !pathMounted("/proc") {
		t.Fatal("expected / or /proc to appear in mountinfo")
	}
}

func TestFirstBin(t *testing.T) {
	if firstBin("/no/such/bin", "/bin/true", "/usr/bin/true") == "" {
		t.Fatal("true binary should resolve")
	}
}
