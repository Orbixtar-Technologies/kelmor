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

func TestRetireWebsiteKeepsHomeAndPool(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.CreateLinuxUser("keep42", 20021, 20021, "/home/keep42", "/usr/sbin/nologin"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyWebsite("site-addon", "keep42", "blog.keep.test", "/home/keep42/blog.keep.test", "php", "", "", false, true, false, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyPHPPool("keep42", "8.3", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/nginx/panel-sites/site-addon.conf")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.retireWebsite("site-addon", "keep42"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/nginx/panel-sites/site-addon.conf")); !os.IsNotExist(err) {
		t.Fatal("vhost remains")
	}
	if _, err := os.Stat(filepath.Join(root, "home/keep42")); err != nil {
		t.Fatal("home removed")
	}
	if _, err := os.Stat(filepath.Join(root, "etc/php/8.3/fpm/pool.d/panel-keep42.conf")); err != nil {
		t.Fatal("php pool removed")
	}
	if _, err := os.Stat(filepath.Join(root, "home/keep42/blog.keep.test")); err != nil {
		t.Fatal("docroot removed")
	}
}

func TestRetireDomainRemovesZoneKeepsHome(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.CreateLinuxUser("keep43", 20022, 20022, "/home/keep43", "/usr/sbin/nologin"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyWebsite("site-gone", "keep43", "gone.keep.test", "/home/keep43/gone.keep.test", "php", "", "", false, true, false, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyDNSZone("gone.keep.test", "$TTL 60\n@ IN SOA ns1.gone.keep.test. hostmaster.gone.keep.test. (1 3600 3600 3600 60)\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.retireDomain("keep43", "gone.keep.test", []string{"site-gone"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/nginx/panel-sites/site-gone.conf")); !os.IsNotExist(err) {
		t.Fatal("vhost remains")
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/dns/zones/gone.keep.test.zone")); !os.IsNotExist(err) {
		t.Fatal("zone remains")
	}
	if _, err := os.Stat(filepath.Join(root, "home/keep43")); err != nil {
		t.Fatal("home removed")
	}
}
