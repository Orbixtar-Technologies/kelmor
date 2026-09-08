package phases

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyWebStackWritesConnZone(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := applyWebStack(Config{Dev: true}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "var/panel/host/etc/nginx/conf.d/panel-conn-limit.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" || !containsAll(string(b), "limit_conn_zone") {
		t.Fatal(string(b))
	}
}

func containsAll(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestRejectUnknownPackage(t *testing.T) {
	if err := InstallPackages([]string{"nginx; rm -rf /"}); err == nil {
		t.Fatal("must reject")
	}
	if err := InstallPackages([]string{"totally-unknown-pkg"}); err == nil {
		t.Fatal("must reject")
	}
}
