package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

const keySize = 32

type Box struct {
	mu      sync.RWMutex
	key     []byte
	version uint32
}

func LoadOrCreate(path string) (*Box, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		key := make([]byte, keySize)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(dirOf(path), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, key, 0o640); err != nil {
			return nil, err
		}
		b = key
	}
	if len(b) != keySize {
		return nil, fmt.Errorf("master key must be %d bytes", keySize)
	}
	return &Box{key: b, version: 1}, nil
}

func FromBytes(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, errors.New("invalid master key length")
	}
	cp := make([]byte, keySize)
	copy(cp, key)
	return &Box{key: cp, version: 1}, nil
}

func (b *Box) Encrypt(plain []byte) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 4, 4+len(nonce)+len(plain)+gcm.Overhead())
	binary.BigEndian.PutUint32(out, b.version)
	out = append(out, nonce...)
	out = append(out, gcm.Seal(nil, nonce, plain, versionAAD(b.version))...)
	return out, nil
}

func (b *Box) Decrypt(blob []byte) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(blob) < 4 {
		return nil, errors.New("ciphertext too short")
	}
	ver := binary.BigEndian.Uint32(blob[:4])
	if ver != b.version {
		return nil, fmt.Errorf("unsupported key version %d", ver)
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	rest := blob[4:]
	if len(rest) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := rest[:nonceSize], rest[nonceSize:]
	return gcm.Open(nil, nonce, ct, versionAAD(ver))
}

func (b *Box) Fingerprint() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	sum := sha256.Sum256(b.key)
	return fmt.Sprintf("%x", sum[:8])
}

func versionAAD(v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return buf
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
