package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hosting-panel/panel/agent/operations"
)

func accountKeyPath(agent *operations.Host) string {
	if agent != nil && agent.Root != "" {
		return filepath.Join(agent.Root, "var/lib/panel/secrets/acme-account.pem")
	}
	if d := os.Getenv("PANEL_STATE_DIR"); d != "" {
		return filepath.Join(d, "control", "acme-account.pem")
	}
	return "/var/lib/panel/control/acme-account.pem"
}

func parseAccountKey(b []byte) *ecdsa.PrivateKey {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil
	}
	return key
}

func loadOrCreateAccountKey(agent *operations.Host) (*ecdsa.PrivateKey, error) {
	path := accountKeyPath(agent)
	if b, err := os.ReadFile(path); err == nil {
		if key := parseAccountKey(b); key != nil {
			return key, nil
		}
	}
	if os.Getenv("PANEL_STATE_DIR") == "" {
		if key := parseAccountKey(mustRead("/var/lib/panel/secrets/acme-account.pem")); key != nil {
			return key, nil
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
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := persistAccountKey(agent, path, pemBytes); err != nil {
		return nil, err
	}
	return key, nil
}

func persistAccountKey(agent *operations.Host, path string, pemBytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err == nil {
		if err := os.WriteFile(path, pemBytes, 0o600); err == nil {
			return nil
		}
	}
	if agent == nil {
		return fmt.Errorf("persist ACME account key: %s not writable", path)
	}
	_, err := agent.Dispatch(context.Background(), operations.Request{
		Method: "ApplyFile",
		Params: mustJSON(map[string]any{
			"path": path, "content_b64": base64.StdEncoding.EncodeToString(pemBytes), "mode": 0o600,
		}),
	})
	if err != nil {
		return fmt.Errorf("persist ACME account key: %w", err)
	}
	return nil
}

func mustRead(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
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
