package update

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSHA256Hi(t *testing.T) {
	sum := sha256.Sum256([]byte("hi"))
	if hex.EncodeToString(sum[:]) == "" {
		t.Fatal()
	}
}
