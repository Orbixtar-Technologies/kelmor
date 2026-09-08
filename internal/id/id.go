package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	last uint64
	seq  uint16
)

// New returns a UUIDv7 string.
func New() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	mu.Lock()
	if ms == last {
		seq++
	} else {
		last = ms
		seq = 0
	}
	s := seq
	mu.Unlock()

	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = 0x70 | byte((s>>8)&0x0f)
	b[7] = byte(s)
	_, _ = rand.Read(b[8:])
	b[8] = (b[8] & 0x3f) | 0x80
	return format(b)
}

func Parse(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 36 {
		return "", fmt.Errorf("invalid id")
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return "", fmt.Errorf("invalid id")
	}
	hexpart := strings.ReplaceAll(s, "-", "")
	if _, err := hex.DecodeString(hexpart); err != nil {
		return "", fmt.Errorf("invalid id")
	}
	return s, nil
}

func format(b [16]byte) string {
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
