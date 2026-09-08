package secret

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	b, err := FromBytes(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := b.Encrypt([]byte("relay-password"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := b.Decrypt(ct)
	if err != nil || string(pt) != "relay-password" {
		t.Fatalf("%q %v", pt, err)
	}
}
