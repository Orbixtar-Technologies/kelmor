package acme

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
)

func TestLoadOrCreateAccountKeyStable(t *testing.T) {
	h := &operations.Host{Root: t.TempDir()}
	a, err := loadOrCreateAccountKey(h)
	if err != nil {
		t.Fatal(err)
	}
	b, err := loadOrCreateAccountKey(h)
	if err != nil {
		t.Fatal(err)
	}
	da, _ := x509.MarshalECPrivateKey(a)
	db, _ := x509.MarshalECPrivateKey(b)
	if !bytes.Equal(da, db) {
		t.Fatal("ACME account key must persist")
	}
	if _, err := os.Stat(accountKeyPath(h)); err != nil {
		t.Fatal(err)
	}
}

func TestParseLeafNotAfter(t *testing.T) {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("nope")}
	if ParseLeafNotAfter(pem.EncodeToMemory(block)) != nil {
		t.Fatal("garbage")
	}
}
