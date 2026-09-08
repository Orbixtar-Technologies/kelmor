package validate

import "testing"

func TestNormalizeDomain(t *testing.T) {
	ok, err := NormalizeDomain("Example.COM.")
	if err != nil || ok != "example.com" {
		t.Fatalf("got %q %v", ok, err)
	}
	if _, err := NormalizeDomain("nope"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NormalizeDomain("bad domain.com"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUsername(t *testing.T) {
	if err := Username("acme42"); err != nil {
		t.Fatal(err)
	}
	if err := Username("root"); err == nil {
		t.Fatal("reserved")
	}
	if err := Username("1bad"); err == nil {
		t.Fatal("must start with letter")
	}
}
