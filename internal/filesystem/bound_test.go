package filesystem

import "testing"

func TestAccountPath(t *testing.T) {
	p, err := AccountPath("acme42", "public_html/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if p != "/home/acme42/public_html/index.html" {
		t.Fatal(p)
	}
	if _, err := AccountPath("acme42", "../etc/passwd"); err == nil {
		t.Fatal("expected deny")
	}
}
