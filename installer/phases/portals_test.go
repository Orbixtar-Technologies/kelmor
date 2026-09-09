package phases

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortalIsBuilt(t *testing.T) {
	built := `<!doctype html><html><head><title>Server Portal</title>
<script type="module" src="/assets/index-abc.js"></script></head>
<body><div id="root"></div></body></html>`
	if !portalIsBuilt(built) {
		t.Fatal("expected built SPA html")
	}
	placeholder := `<!doctype html><html><head><title>Server Portal</title></head>
<body><h1>Server Portal</h1><p>Built portal assets were not found</p></body></html>`
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
		title := "Server Portal"
		if name == "account" {
			title = "Account Portal"
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
}
