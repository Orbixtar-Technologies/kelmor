package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
)

func NewOpaqueToken() (plain string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, b); err != nil {
		return "", nil, err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(plain))
	return plain, sum[:], nil
}

func HashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

func TokenPrefix(plain string) string {
	if len(plain) < 12 {
		return plain
	}
	return plain[:12]
}
