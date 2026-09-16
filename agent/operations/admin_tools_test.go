package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyAdminToolsWritesVhostsAndOverlays(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if _, err := h.applyAdminTools("acme.test", nil); err != nil {
		t.Fatal(err)
	}
	webmail, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/webmail-acme.test.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(webmail), "server_name webmail.acme.test;") {
		t.Fatalf("webmail vhost: %s", webmail)
	}
	if !strings.Contains(string(webmail), "root /usr/share/roundcube") {
		t.Fatalf("webmail root: %s", webmail)
	}
	pma, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/phpmyadmin-acme.test.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pma), "server_name phpmyadmin.acme.test;") {
		t.Fatalf("phpmyadmin vhost: %s", pma)
	}
	overlay, err := os.ReadFile(filepath.Join(root, "etc/phpmyadmin/conf.d/panel-host.inc.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overlay), "AllowArbitraryServer") {
		t.Fatalf("phpmyadmin overlay: %s", overlay)
	}
	rc, err := os.ReadFile(filepath.Join(root, "etc/roundcube/config.panel.inc.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rc), "imap_host") || !strings.Contains(string(rc), "smtp_user") {
		t.Fatalf("roundcube overlay: %s", rc)
	}
	main, err := os.ReadFile(filepath.Join(root, "etc/roundcube/config.inc.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main), "/etc/roundcube/config.panel.inc.php") {
		t.Fatalf("roundcube include: %s", main)
	}
	if _, err := os.Stat(filepath.Join(root, "usr/share/roundcube/index.php")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyAdminToolsBindsTLSWhenCertExists(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	if err := os.MkdirAll(filepath.Join(root, "var/lib/panel/certs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "var/lib/panel/certs/shop.test.crt"), []byte("crt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "var/lib/panel/certs/shop.test.key"), []byte("key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyAdminTools("shop.test", []string{"roundcube"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "etc/nginx/panel-sites/webmail-shop.test.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "listen 443 ssl") {
		t.Fatalf("missing HTTPS: %s", body)
	}
	if !strings.Contains(string(body), "return 301 https://$host$request_uri") {
		t.Fatalf("missing HTTP redirect: %s", body)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/nginx/panel-sites/phpmyadmin-shop.test.conf")); !os.IsNotExist(err) {
		t.Fatal("phpmyadmin should stay unpublished when not requested")
	}
}

func TestApplyAdminToolsKeepsExistingRoundcubeConfig(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	main := filepath.Join(root, "etc/roundcube/config.inc.php")
	if err := os.MkdirAll(filepath.Dir(main), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "<?php\ninclude(\"/etc/roundcube/debian-db-roundcube.php\");\n$config['skin'] = 'elastic';\n"
	if err := os.WriteFile(main, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyAdminTools("keep.test", []string{"roundcube"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, "debian-db-roundcube.php") {
		t.Fatalf("lost package db include: %s", body)
	}
	if !strings.Contains(body, "/etc/roundcube/config.panel.inc.php") {
		t.Fatalf("missing panel include: %s", body)
	}
}

func TestSystemPathRejectsUnmanagedRoots(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.systemPath("/etc/passwd"); err == nil {
		t.Fatal("must reject unmanaged system paths")
	}
	if _, err := h.resolve("/usr/share/phpmyadmin"); err == nil {
		t.Fatal("resolve must still reject package roots")
	}
}
