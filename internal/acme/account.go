package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hosting-panel/panel/agent/operations"
)

func accountKeyPath(agent *operations.Host) string {
	p := "/var/lib/panel/secrets/acme-account.pem"
	if agent != nil && agent.Root != "" {
		return filepath.Join(agent.Root, "var/lib/panel/secrets/acme-account.pem")
	}
	return p
}

func loadOrCreateAccountKey(agent *operations.Host) (*ecdsa.PrivateKey, error) {
	path := accountKeyPath(agent)
	if b, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(b)
		if block != nil {
			if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return key, nil
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		return nil, fmt.Errorf("persist ACME account key: %w", err)
	}
	return key, nil
}

func ParseLeafNotAfter(pemBytes []byte) *x509.Certificate {
	for {
		var block *pem.Block
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			return nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		return c
	}
}
