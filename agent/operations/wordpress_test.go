package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallWordPressSandbox(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	st, err := h.installWordPress(WordPressInstall{
		Username:      "acme42",
		DocumentRoot:  "/home/acme42/public_html",
		DBName:        "acme42_wp",
		DBUser:        "acme42_u",
		DBPassword:    "secret-db",
		Title:         "Acme Site",
		AdminUser:     "acmeadmin",
		AdminPassword: "AdminPass!2026",
		AdminEmail:    "owner@acme.test",
		SiteURL:       "http://acme.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !st.OK {
		t.Fatalf("%+v", st)
	}
	cfg := filepath.Join(h.Root, "home/acme42/public_html/wp-config.php")
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "acme42_wp") || !strings.Contains(string(b), "secret-db") {
		t.Fatalf("%s", b)
	}
	if !strings.Contains(string(b), "WP_HOME") || !strings.Contains(string(b), "http://acme.test") {
		t.Fatalf("site url: %s", b)
	}
	if _, err := os.Stat(filepath.Join(h.Root, "home/acme42/public_html/index.php")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallWordPressRejectsEscape(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.installWordPress(WordPressInstall{
		Username:      "acme42",
		DocumentRoot:  "/home/other/public_html",
		DBName:        "acme42_wp",
		DBUser:        "acme42_u",
		DBPassword:    "secret-db",
		Title:         "Acme",
		AdminUser:     "acmeadmin",
		AdminPassword: "AdminPass!2026",
		AdminEmail:    "o@acme.test",
	})
	if err == nil {
		t.Fatal("expected account boundary error")
	}
}
