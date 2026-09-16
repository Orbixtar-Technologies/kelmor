package operations

import (
	"os"
	"path/filepath"
	"testing"
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
}

func TestApplyACMEChallengeRejectsTokenPath(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.applyACMEChallenge("../x", "x"); err == nil {
		t.Fatal("expected reject")
	}
}
