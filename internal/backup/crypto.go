package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"filippo.io/age"
	"github.com/hosting-panel/panel/internal/pkg/secret"
)

type keySlot struct {
	id        string
	identity  age.Identity
	recipient age.Recipient
	version   uint32
}

type KeyRing struct {
	current *keySlot
	slots   []*keySlot
}

func KeyRingFromBox(box *secret.Box) (*KeyRing, error) {
	if box == nil {
		return nil, fmt.Errorf("backup key is required")
	}
	slot, err := slotFromBox(box)
	if err != nil {
		return nil, err
	}
	return &KeyRing{current: slot, slots: []*keySlot{slot}}, nil
}

func KeyRingFromBoxes(current *secret.Box, previous ...*secret.Box) (*KeyRing, error) {
	ring, err := KeyRingFromBox(current)
	if err != nil {
		return nil, err
	}
	for _, box := range previous {
		slot, err := slotFromBox(box)
		if err != nil {
			return nil, err
		}
		ring.slots = append(ring.slots, slot)
	}
	return ring, nil
}

func slotFromBox(box *secret.Box) (*keySlot, error) {
	scalar := box.Derive("kelmor-hpm3-x25519")
	encoded, err := encodeAgeSecretKey(scalar)
	if err != nil {
		return nil, err
	}
	id, err := age.ParseX25519Identity(encoded)
	if err != nil {
		return nil, err
	}
	rec := id.Recipient()
	sum := sha256.Sum256([]byte(rec.String()))
	return &keySlot{
		id:        hex.EncodeToString(sum[:]),
		identity:  id,
		recipient: rec,
		version:   box.Version(),
	}, nil
}

func (k *KeyRing) CurrentIdentity() string {
	if k == nil || k.current == nil {
		return ""
	}
	return k.current.id
}

func (k *KeyRing) CurrentVersion() uint32 {
	if k == nil || k.current == nil {
		return 0
	}
	return k.current.version
}

func (k *KeyRing) recipients() []age.Recipient {
	if k == nil || k.current == nil {
		return nil
	}
	return []age.Recipient{k.current.recipient}
}

func (k *KeyRing) identities() []age.Identity {
	if k == nil {
		return nil
	}
	out := make([]age.Identity, 0, len(k.slots))
	for _, slot := range k.slots {
		out = append(out, slot.identity)
	}
	return out
}

func encodeAgeSecretKey(scalar []byte) (string, error) {
	encoded, err := bech32Encode("age-secret-key-", scalar)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(encoded), nil
}

var bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var bech32Gen = []uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}

func bech32Encode(hrp string, data []byte) (string, error) {
	values, err := bech32ConvertBits(data, 8, 5, true)
	if err != nil {
		return "", err
	}
	hrp = strings.ToLower(hrp)
	var out strings.Builder
	out.WriteString(hrp)
	out.WriteByte('1')
	for _, v := range values {
		out.WriteByte(bech32Charset[v])
	}
	for _, v := range bech32Checksum(hrp, values) {
		out.WriteByte(bech32Charset[v])
	}
	return out.String(), nil
}

func bech32Checksum(hrp string, data []byte) []byte {
	values := append(bech32HRPExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	mod := bech32Polymod(values) ^ 1
	ret := make([]byte, 6)
	for i := range ret {
		ret[i] = byte(mod>>(5*(5-i))) & 31
	}
	return ret
}

func bech32HRPExpand(hrp string) []byte {
	h := []byte(hrp)
	ret := make([]byte, 0, len(h)*2+1)
	for _, c := range h {
		ret = append(ret, c>>5)
	}
	ret = append(ret, 0)
	for _, c := range h {
		ret = append(ret, c&31)
	}
	return ret
}

func bech32Polymod(values []byte) uint32 {
	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk & 0x1ffffff) << 5
		chk ^= uint32(v)
		for i := 0; i < 5; i++ {
			if (top>>i)&1 == 1 {
				chk ^= bech32Gen[i]
			}
		}
	}
	return chk
}

func bech32ConvertBits(data []byte, frombits, tobits byte, pad bool) ([]byte, error) {
	var ret []byte
	acc := uint32(0)
	bits := byte(0)
	maxv := byte(1<<tobits - 1)
	for _, value := range data {
		acc = acc<<frombits | uint32(value)
		bits += frombits
		for bits >= tobits {
			bits -= tobits
			ret = append(ret, byte(acc>>bits)&maxv)
		}
	}
	if pad && bits > 0 {
		ret = append(ret, byte(acc<<(tobits-bits))&maxv)
	}
	return ret, nil
}
