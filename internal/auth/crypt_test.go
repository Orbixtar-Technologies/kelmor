package auth

import "testing"

func TestSHA512Crypt(t *testing.T) {
	h, err := SHA512Crypt("FtpPass!2026")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) < 20 || h[:3] != "$6$" {
		t.Fatalf("hash %q", h)
	}
}
