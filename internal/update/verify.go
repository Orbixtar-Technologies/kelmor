package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Manifest struct {
	Release   string            `json:"release"`
	Channel   string            `json:"channel"`
	Files     map[string]string `json:"files"`
	Signature string            `json:"signature"`
	PublicKey string            `json:"public_key,omitempty"`
}

func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) UnsignedPayload() ([]byte, error) {
	cp := *m
	cp.Signature = ""
	return json.Marshal(cp)
}

func Verify(m *Manifest, pub ed25519.PublicKey) error {
	if m.Release == "" || m.Channel == "" {
		return fmt.Errorf("manifest missing release metadata")
	}
	if strings.EqualFold(m.Signature, "unsigned-development") {
		return fmt.Errorf("unsigned downgrades are refused")
	}
	sig, err := hex.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid manifest signature")
	}
	body, err := canonical(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, body, sig) {
		return fmt.Errorf("manifest signature mismatch")
	}
	return nil
}

func VerifyFileHashes(m *Manifest, dir string) error {
	for rel, want := range m.Files {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != want {
			return fmt.Errorf("hash mismatch for %s", rel)
		}
	}
	return nil
}

func ParsePublicKey(hexKey string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key")
	}
	return ed25519.PublicKey(b), nil
}

func canonical(m *Manifest) ([]byte, error) {
	type wire struct {
		Release string            `json:"release"`
		Channel string            `json:"channel"`
		Files   map[string]string `json:"files"`
	}
	return json.Marshal(wire{Release: m.Release, Channel: m.Channel, Files: m.Files})
}
