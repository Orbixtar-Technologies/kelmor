package policy

import "testing"

func TestValidateManagedPath(t *testing.T) {
	if _, err := ValidateManagedPath("/home/acme42/public_html"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateManagedPath("/etc/passwd"); err == nil {
		t.Fatal("should reject")
	}
	if _, err := ValidateManagedPath("/home/../etc/passwd"); err == nil {
		t.Fatal("should reject traversal")
	}
}

func TestWithinAccount(t *testing.T) {
	if _, err := WithinAccount("acme42", "/home/acme42/apps"); err != nil {
		t.Fatal(err)
	}
	if _, err := WithinAccount("acme42", "/home/other/apps"); err == nil {
		t.Fatal("cross-account")
	}
}
