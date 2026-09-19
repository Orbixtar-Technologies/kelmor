package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicPortalHostnamePrefersConfiguredNameOverIP(t *testing.T) {
	t.Setenv("PANEL_HOSTNAME", "kelmor.host")
	req := httptestRequest("150.239.113.59:8443")
	if got := publicPortalHostname(req); got != "kelmor.host" {
		t.Fatalf("public hostname %q", got)
	}
	if got := controlPortalURL(req); got != "https://kelmor.host:2083/" {
		t.Fatalf("control url %q", got)
	}
}

func TestPublicPortalHostnameUsesRequestHostWhenUnconfigured(t *testing.T) {
	t.Setenv("PANEL_HOSTNAME", "")
	t.Setenv("PANEL_STATE_DIR", t.TempDir())
	req := httptestRequest("kelmor.host:2087")
	if got := controlPortalURL(req); got != "https://kelmor.host:2083/" {
		t.Fatalf("control url %q", got)
	}
}

func TestControlPortalURLOmitsLoopback(t *testing.T) {
	t.Setenv("PANEL_HOSTNAME", "")
	t.Setenv("PANEL_STATE_DIR", t.TempDir())
	if got := controlPortalURL(httptestRequest("127.0.0.1:18443")); got != "" {
		t.Fatalf("loopback control url %q", got)
	}
}

func TestPortalHostnameFromStateFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PANEL_HOSTNAME", "")
	t.Setenv("PANEL_STATE_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "portal-hostname"), []byte("panel.example.net\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := portalHostnameFromConfig(); got != "panel.example.net" {
		t.Fatalf("state file hostname %q", got)
	}
}

func TestAdminToolURLIsHTTPS(t *testing.T) {
	if got := adminToolURL("webmail", "shop.test"); got != "https://webmail.shop.test/" {
		t.Fatalf("webmail %q", got)
	}
	if got := adminToolURL("phpmyadmin", " shop.test "); got != "https://phpmyadmin.shop.test/" {
		t.Fatalf("phpmyadmin %q", got)
	}
	if adminToolURL("webmail", "") != "" {
		t.Fatal("empty domain should not invent a host")
	}
}

func httptestRequest(host string) *http.Request {
	req, err := http.NewRequest(http.MethodGet, "http://"+host+"/", nil)
	if err != nil {
		panic(err)
	}
	req.Host = host
	req.RemoteAddr = "127.0.0.1:1"
	return req
}
