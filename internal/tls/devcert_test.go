package tls

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestSelfSignedNamesIncludesAliases(t *testing.T) {
	cert, _, err := SelfSignedNames([]string{"a.test", "www.a.test"}, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(cert)
	if block == nil {
		t.Fatal("pem")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "a.test" {
		t.Fatalf("cn %s", leaf.Subject.CommonName)
	}
	want := map[string]bool{"a.test": false, "www.a.test": false}
	for _, n := range leaf.DNSNames {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, ok := range want {
		if !ok {
			t.Fatalf("missing SAN %s in %v", n, leaf.DNSNames)
		}
	}
}

func TestSelfSignedNamesIncludesLoopbackIP(t *testing.T) {
	cert, _, err := SelfSignedNames([]string{"panel.example.net", "127.0.0.1"}, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(cert)
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ip := range leaf.IPAddresses {
		if ip.String() == "127.0.0.1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing 127.0.0.1 SAN in %v", leaf.IPAddresses)
	}
}
