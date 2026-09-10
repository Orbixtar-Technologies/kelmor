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
	if !strings.Contains(string(conf), "ssl_certificate") ||
		!strings.Contains(string(conf), "listen 8443 ssl") ||
		!strings.Contains(string(conf), "location /updates/") ||
		!strings.Contains(string(conf), "proxy_set_header X-Forwarded-Host $http_host;") {
		t.Fatalf("portal nginx missing TLS or update feed: %s", conf)
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

func TestVerifyPortalsRejectsLegacyBrand(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Hostname: "panel.example.net", Root: dir}
	for _, rel := range []string{
		"usr/local/panel/share/portals/server",
		"usr/local/panel/share/portals/account",
		"etc/nginx/panel-sites",
		"var/lib/panel/certs",
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	legacy := `<!doctype html><html><head><title>Server Portal</title>
<script type="module" src="/assets/index.js"></script></head><body><div id="root"></div></body></html>`
	control := `<!doctype html><html><head><title>Kelmor Control</title>
<script type="module" src="/assets/index.js"></script></head><body><div id="root"></div></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "usr/local/panel/share/portals/server/index.html"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "usr/local/panel/share/portals/account/index.html"), []byte(control), 0o644); err != nil {
		t.Fatal(err)
	}
	tls := "server {\n    listen 8443 ssl;\n    ssl_certificate /tmp/x.crt;\n    ssl_certificate_key /tmp/x.key;\n    root /usr/local/panel/share/portals/server;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "etc/nginx/panel-sites/90-server-portal.conf"), []byte(tls), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "etc/nginx/panel-sites/91-account-portal.conf"), []byte(strings.ReplaceAll(tls, "8443", "8444")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "var/lib/panel/certs/panel-portals.crt"), []byte("CERT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "var/lib/panel/certs/panel-portals.key"), []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifyPortals(cfg)
	if err == nil || !strings.Contains(err.Error(), "legacy chrome") {
		t.Fatalf("expected legacy chrome reject, got %v", err)
	}
}

func TestPortalAssetRootPrefersTreeBuild(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	fresh := filepath.Join(dir, "portals", "server", "dist")
	stale := filepath.Join(dir, "dist", "share", "portals", "server")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	freshHTML := `<!doctype html><html><head><title>Kelmor Director</title></head><body><div id="root"></div></body></html>`
	staleHTML := `<!doctype html><html><head><title>Server Portal</title></head><body><div id="root"></div></body></html>`
	if err := os.WriteFile(filepath.Join(fresh, "index.html"), []byte(freshHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "index.html"), []byte(staleHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	got := portalAssetRoot(Config{Root: dir, Dev: true}, "server")
	abs, err := filepath.Abs(got)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if abs != want {
		t.Fatalf("preferred %q want %q", abs, want)
	}
}
