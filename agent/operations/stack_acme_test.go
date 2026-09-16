package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/configuration"
)

func TestApplyACMEChallengeWritesWebroots(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.applyACMEChallenge("tok-1", "challenge-body"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"/var/lib/panel/acme-www/.well-known/acme-challenge/tok-1",
		"/var/www/panel-acme/.well-known/acme-challenge/tok-1",
	} {
		b, err := os.ReadFile(filepath.Join(h.Root, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if string(b) != "challenge-body" {
			t.Fatalf("%s: %q", rel, b)
		}
	}
	st, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/acme-www"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o005 == 0 {
		t.Fatalf("acme-www must be world-traversable, got %o", st.Mode().Perm())
	}
	acme, err := os.ReadFile(filepath.Join(h.Root, "etc/nginx/panel-sites/00-acme.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(acme), configuration.ACMEHTTP01Root) {
		t.Fatalf("00-acme.conf: %s", acme)
	}
}

func TestApplyACMEChallengeRetargetsStaleVhosts(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	dir := filepath.Join(h.Root, "etc/nginx/panel-sites")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "server {\n    listen 80;\n    server_name orbixtar.dpdns.org;\n    location ^~ /.well-known/acme-challenge/ { root /var/lib/panel/acme-www; default_type text/plain; }\n    location / { return 404;\n}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "site-1.conf"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyACMEChallenge("tok-2", "challenge-body"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "site-1.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "root "+configuration.ACMEHTTP01Root+";") {
		t.Fatalf("stale ACME root not rewritten: %s", body)
	}
	if strings.Contains(string(body), "root /var/lib/panel/acme-www;") {
		t.Fatalf("stale acme-www root remains: %s", body)
	}
}

func TestApplyACMEChallengeRejectsTokenPath(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.applyACMEChallenge("../x", "x"); err == nil {
		t.Fatal("expected reject")
	}
}
