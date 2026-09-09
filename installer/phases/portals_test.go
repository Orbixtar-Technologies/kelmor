package phases

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortalIsBuilt(t *testing.T) {
	built := `<!doctype html><html><head><title>Kelmor Director</title>
<script type="module" src="/assets/index-abc.js"></script></head>
<body><div id="root"></div></body></html>`
	if !portalIsBuilt(built) {
		t.Fatal("expected built SPA html")
	}
	placeholder := `<!doctype html><html><head><title>Kelmor Director</title></head>
<body><h1>Kelmor Director</h1><p>Built portal assets were not found</p></body></html>`
	if portalIsBuilt(placeholder) {
		t.Fatal("placeholder must not count as built")
	}
}

func TestLivePortalsRequireBuiltAssets(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	cfg := Config{Hostname: "panel.example.net", AdminEmail: "ops@example.net", Root: dir}
	if err := applyPortals(cfg); err == nil {
		t.Fatal("live install without SPA assets should fail")
	}
	for _, name := range []string{"server", "account"} {
		dest := filepath.Join(dir, "dist", "share", "portals", name)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		title := "Kelmor Director"
		if name == "account" {
			title = "Kelmor Control"
		}
		html := `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>` + title + `</title>
<script type="module" src="/assets/index.js"></script></head><body><div id="root"></div></body></html>`
		if err := os.WriteFile(filepath.Join(dest, "index.html"), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := applyPortals(cfg); err != nil {
		t.Fatalf("apply with built assets: %v", err)
	}
	if err := verifyPortals(cfg); err != nil {
		t.Fatalf("verify: %v", err)
	}
	conf, err := os.ReadFile(filepath.Join(dir, "etc/nginx/panel-sites/90-server-portal.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conf), "ssl_certificate") || !strings.Contains(string(conf), "listen 8443 ssl") {
		t.Fatalf("portal nginx missing TLS: %s", conf)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "var/lib/panel/portal-hostname")); err != nil || string(b) != "panel.example.net\n" {
		t.Fatalf("portal-hostname: %s %v", b, err)
	}
	hostCert := filepath.Join(dir, "var/lib/panel/certs/panel.example.net.crt")
	hostKey := filepath.Join(dir, "var/lib/panel/certs/panel.example.net.key")
	if err := os.WriteFile(hostCert, []byte("CERT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostKey, []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePortalNginx(cfg, "90-server-portal.conf", 8443, "usr/local/panel/share/portals/server"); err != nil {
		t.Fatal(err)
	}
	conf, err = os.ReadFile(filepath.Join(dir, "etc/nginx/panel-sites/90-server-portal.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conf), "server_name panel.example.net") || !strings.Contains(string(conf), hostCert) {
		t.Fatalf("missing hostname ACME vhost: %s", conf)
	}
}

func TestVerifyPortalsRequiresTLS(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", Root: dir}
	for _, rel := range []string{
		"usr/local/panel/share/portals/server",
		"usr/local/panel/share/portals/account",
		"etc/nginx/panel-sites",
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	serverHTML := `<!doctype html><html><head><title>Kelmor Director</title>
<script type="module" src="/assets/index.js"></script></head><body><div id="root"></div></body></html>`
	accountHTML := `<!doctype html><html><head><title>Kelmor Control</title>
<script type="module" src="/assets/index.js"></script></head><body><div id="root"></div></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "usr/local/panel/share/portals/server/index.html"), []byte(serverHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "usr/local/panel/share/portals/account/index.html"), []byte(accountHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	plain := "server {\n    listen 8443;\n    root /usr/local/panel/share/portals/server;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "etc/nginx/panel-sites/90-server-portal.conf"), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "etc/nginx/panel-sites/91-account-portal.conf"), []byte(strings.ReplaceAll(plain, "8443", "8444")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyPortals(cfg); err == nil {
		t.Fatal("HTTP-only portal nginx must fail verify")
	}
}
