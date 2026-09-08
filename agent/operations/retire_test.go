package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetireAccountRemovesHostArtifacts(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.CreateLinuxUser("gone42", 20020, 20020, "/home/gone42", "/usr/sbin/nologin"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyWebsite("site-1", "gone42", "gone.test", "/home/gone42/public_html", "php", "", "", false, true, false, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyPHPPool("gone42", "8.3", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := h.createMailboxHome("gone.test", "info", 20020, 20020); err != nil {
		t.Fatal(err)
	}
	if _, err := h.setQuota("gone42", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := h.enforceAccountBandwidth("gone42", true); err != nil {
		t.Fatal(err)
	}
	if err := h.applyCgroupLimits("gone42", 50, 64<<20, 20, 80, 250); err != nil {
		t.Fatal(err)
	}
	if _, err := h.retireAccount("gone42", []string{"site-1"}, []string{"gone.test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "home/gone42")); !os.IsNotExist(err) {
		t.Fatalf("home remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/nginx/panel-sites/site-1.conf")); !os.IsNotExist(err) {
		t.Fatal("vhost remains")
	}
	if _, err := os.Stat(filepath.Join(root, "etc/php/8.3/fpm/pool.d/panel-gone42.conf")); !os.IsNotExist(err) {
		t.Fatal("php pool remains")
	}
	if _, err := os.Stat(filepath.Join(root, "var/vmail/gone.test")); !os.IsNotExist(err) {
		t.Fatal("mail tree remains")
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/quotas/gone42")); !os.IsNotExist(err) {
		t.Fatal("quota file remains")
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/bandwidth/gone42")); !os.IsNotExist(err) {
		t.Fatal("bandwidth dir remains")
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/cgroup/gone42")); !os.IsNotExist(err) {
		t.Fatal("cgroup spec remains")
	}
}
