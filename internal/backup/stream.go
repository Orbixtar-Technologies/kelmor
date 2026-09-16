package backup

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/releaseversion"
	"github.com/hosting-panel/panel/internal/store"
)

const (
	FormatHPM3     = 3
	ComponentHome  = "home"
	hpm3Magic      = "HPM3\n"
	ageMagic       = "age-encryption.org/v1"
	maxHeaderBytes = 1 << 20
	copyBufSize    = 64 << 10
)

type ConsistencyAnchor struct {
	Fence      int64    `json:"fence"`
	Level      string   `json:"level"`
	Quiesced   []string `json:"quiesced,omitempty"`
	SnapshotAt string   `json:"snapshot_at,omitempty"`
}

type Component struct {
	Name string
	Data []byte
	Size int64
	Hash string
	Open func() (io.ReadCloser, int64, error)
}

type ArchiveReader struct {
	Manifest Manifest
	plain    io.Reader
	closers  []io.Closer
	seen     map[string]bool
	index    int
	legacy   []byte
}

func SealHPM3(ctx context.Context, box *secret.Box, repo Repository, acc *store.Account, parts []Component, expected []string, anchor ConsistencyAnchor) (Manifest, string, error) {
	created := time.Now().UTC().Format(time.RFC3339)
	return SealHPM3ToKey(ctx, box, repo, acc, ObjectKey(acc, created), parts, expected, anchor)
}

func SealHPM3ToKey(ctx context.Context, box *secret.Box, repo Repository, acc *store.Account, key string, parts []Component, expected []string, anchor ConsistencyAnchor) (Manifest, string, error) {
	ring, err := KeyRingFromBox(box)
	if err != nil {
		return Manifest{}, "", err
	}
	return SealHPM3WithRingToKey(ctx, ring, repo, acc, key, parts, expected, anchor)
}

func SealHPM3WithRing(ctx context.Context, ring *KeyRing, repo Repository, acc *store.Account, parts []Component, expected []string, anchor ConsistencyAnchor) (Manifest, string, error) {
	created := time.Now().UTC().Format(time.RFC3339)
	return SealHPM3WithRingToKey(ctx, ring, repo, acc, ObjectKey(acc, created), parts, expected, anchor)
}

func SealHPM3WithRingToKey(ctx context.Context, ring *KeyRing, repo Repository, acc *store.Account, key string, parts []Component, expected []string, anchor ConsistencyAnchor) (Manifest, string, error) {
	if err := validateAnchor(anchor); err != nil {
		return Manifest{}, "", err
	}
	if err := validateInventory(parts, expected); err != nil {
		return Manifest{}, "", err
	}
	man := Manifest{
		FormatVersion: FormatHPM3,
		AccountID:     acc.ID,
		Username:      acc.Username,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		PanelVersion:  releaseversion.Current(),
		Kind:          "full",
		Files:         map[string]any{"home": acc.HomePath},
		Checksums:     map[string]string{},
		KeyIdentity:   ring.CurrentIdentity(),
		KeyVersion:    ring.CurrentVersion(),
		Consistency:   anchor,
		Expected:      append([]string{}, expected...),
	}
	for _, name := range expected {
		switch {
		case strings.HasPrefix(name, "databases/"):
			_, file, ok := strings.Cut(strings.TrimPrefix(name, "databases/"), "/")
			if ok {
				man.Databases = append(man.Databases, strings.TrimSuffix(file, ".sql"))
			}
		case strings.HasPrefix(name, "mail/"):
			_, local, ok := strings.Cut(strings.TrimPrefix(name, "mail/"), "/")
			if ok {
				man.Mailboxes = append(man.Mailboxes, local)
			}
		}
	}
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- writeHPM3(pw, ring, man, parts)
	}()
	hasher := sha256.New()
	if err := putStream(ctx, repo, key, io.TeeReader(pr, hasher)); err != nil {
		_ = pr.CloseWithError(err)
		<-errCh
		return man, "", err
	}
	if err := <-errCh; err != nil {
		return man, "", err
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	man.Checksums["object"] = sum
	if err := repo.Verify(ctx, key, sum); err != nil {
		return man, "", err
	}
	return man, key, nil
}

func writeHPM3(w *io.PipeWriter, ring *KeyRing, man Manifest, parts []Component) (err error) {
	defer func() {
		if err != nil {
			_ = w.CloseWithError(err)
			return
		}
		_ = w.Close()
	}()
	enc, err := age.Encrypt(w, ring.recipients()...)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := enc.Close(); err == nil {
			err = closeErr
		}
	}()
	header, err := json.Marshal(man)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(enc, hpm3Magic); err != nil {
		return err
	}
	if err := binary.Write(enc, binary.BigEndian, uint32(len(header))); err != nil {
		return err
	}
	if _, err := enc.Write(header); err != nil {
		return err
	}
	for _, part := range parts {
		if err := writeComponent(enc, part); err != nil {
			return err
		}
	}
	return nil
}

func writeComponent(w io.Writer, part Component) error {
	r, size, err := openComponent(part)
	if err != nil {
		return err
	}
	defer r.Close()
	if part.Name == ComponentHome && size == 0 {
		return fmt.Errorf("empty home component")
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(part.Name))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, part.Name); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint64(size)); err != nil {
		return err
	}
	hasher := sha256.New()
	copied, err := io.CopyBuffer(io.MultiWriter(w, hasher), io.LimitReader(r, size), make([]byte, copyBufSize))
	if err != nil {
		return err
	}
	if copied != size {
		return fmt.Errorf("component %s truncated", part.Name)
	}
	sum := hasher.Sum(nil)
	if part.Hash != "" && part.Hash != hex.EncodeToString(sum) {
		return fmt.Errorf("component %s hash mismatch", part.Name)
	}
	_, err = w.Write(sum)
	return err
}

func openComponent(part Component) (io.ReadCloser, int64, error) {
	if part.Open != nil {
		r, size, err := part.Open()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unreadable") {
				return nil, 0, fmt.Errorf("component %s unreadable: %w", part.Name, err)
			}
			return nil, 0, err
		}
		if part.Size > 0 {
			size = part.Size
		}
		return r, size, nil
	}
	body := part.Data
	if body == nil {
		body = []byte{}
	}
	return io.NopCloser(bytes.NewReader(body)), int64(len(body)), nil
}

func validateAnchor(anchor ConsistencyAnchor) error {
	if anchor.Fence == 0 || strings.TrimSpace(anchor.Level) == "" {
		return fmt.Errorf("consistency anchor is required")
	}
	return nil
}

func validateInventory(parts []Component, expected []string) error {
	if len(parts) != len(expected) {
		if len(parts) < len(expected) {
			return fmt.Errorf("missing backup component")
		}
		return fmt.Errorf("duplicate backup component")
	}
	seen := map[string]int{}
	for i, part := range parts {
		if part.Name == "" {
			return fmt.Errorf("component name required")
		}
		if seen[part.Name] > 0 {
			return fmt.Errorf("duplicate backup component %s", part.Name)
		}
		seen[part.Name]++
		if part.Name != expected[i] {
			return fmt.Errorf("component order mismatch: got %s want %s", part.Name, expected[i])
		}
	}
	return nil
}

func OpenAny(ctx context.Context, box *secret.Box, repo Repository, key string) (*ArchiveReader, error) {
	ring, err := KeyRingFromBox(box)
	if err != nil {
		return nil, err
	}
	return openAny(ctx, ring, box, repo, key)
}

func OpenAnyWithRing(ctx context.Context, ring *KeyRing, repo Repository, key string) (*ArchiveReader, error) {
	return openAny(ctx, ring, nil, repo, key)
}

func openAny(ctx context.Context, ring *KeyRing, box *secret.Box, repo Repository, key string) (*ArchiveReader, error) {
	rc, err := getStream(ctx, repo, key)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(rc)
	peek, err := br.Peek(len(ageMagic))
	if err != nil && err != io.EOF {
		_ = rc.Close()
		return nil, err
	}
	if bytes.HasPrefix(peek, []byte(ageMagic)) {
		plain, err := age.Decrypt(br, ring.identities()...)
		if err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("hpm3 decrypt: %w", err)
		}
		man, rest, err := readHPM3Header(plain)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		return &ArchiveReader{Manifest: man, plain: rest, closers: []io.Closer{rc}, seen: map[string]bool{}}, nil
	}
	_ = rc.Close()
	if box == nil {
		return nil, fmt.Errorf("hpm1 backup requires the original box key")
	}
	man, raw, err := OpenArchive(ctx, box, repo, key)
	if err != nil {
		return nil, err
	}
	return &ArchiveReader{
		Manifest: man,
		plain:    bytes.NewReader(raw),
		seen:     map[string]bool{},
		legacy:   raw,
	}, nil
}

func readHPM3Header(r io.Reader) (Manifest, io.Reader, error) {
	magic := make([]byte, len(hpm3Magic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return Manifest{}, nil, err
	}
	if string(magic) != hpm3Magic {
		return Manifest{}, nil, fmt.Errorf("unknown hpm3 magic")
	}
	var headerLen uint32
	if err := binary.Read(r, binary.BigEndian, &headerLen); err != nil {
		return Manifest{}, nil, err
	}
	if headerLen == 0 || headerLen > maxHeaderBytes {
		return Manifest{}, nil, fmt.Errorf("invalid hpm3 header")
	}
	raw := make([]byte, headerLen)
	if _, err := io.ReadFull(r, raw); err != nil {
		return Manifest{}, nil, err
	}
	var man Manifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return Manifest{}, nil, err
	}
	if man.FormatVersion != FormatHPM3 {
		return Manifest{}, nil, fmt.Errorf("unsupported backup format %d", man.FormatVersion)
	}
	return man, r, nil
}

func (a *ArchiveReader) Each(fn func(name string, r io.Reader) error) error {
	if a == nil {
		return fmt.Errorf("archive is required")
	}
	if a.Manifest.FormatVersion != FormatHPM3 {
		if a.legacy == nil {
			return fmt.Errorf("legacy archive payload missing")
		}
		return fn(ComponentHome, bytes.NewReader(a.legacy))
	}
	for {
		name, r, err := a.nextFrame()
		if err == io.EOF {
			if a.index < len(a.Manifest.Expected) {
				return fmt.Errorf("missing backup component")
			}
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(name, r); err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, r); err != nil && err != io.EOF {
			return err
		}
	}
}

func (a *ArchiveReader) nextFrame() (string, io.Reader, error) {
	var nameLen uint32
	if err := binary.Read(a.plain, binary.BigEndian, &nameLen); err != nil {
		return "", nil, err
	}
	if nameLen == 0 || nameLen > 4096 {
		return "", nil, fmt.Errorf("invalid component name")
	}
	nameRaw := make([]byte, nameLen)
	if _, err := io.ReadFull(a.plain, nameRaw); err != nil {
		return "", nil, err
	}
	name := string(nameRaw)
	if a.seen[name] {
		return "", nil, fmt.Errorf("duplicate backup component %s", name)
	}
	a.seen[name] = true
	if a.index >= len(a.Manifest.Expected) || a.Manifest.Expected[a.index] != name {
		return "", nil, fmt.Errorf("component order mismatch")
	}
	a.index++
	var size uint64
	if err := binary.Read(a.plain, binary.BigEndian, &size); err != nil {
		return "", nil, err
	}
	hasher := sha256.New()
	limited := io.LimitReader(a.plain, int64(size))
	pr, pw := io.Pipe()
	go func() {
		_, copyErr := io.CopyBuffer(io.MultiWriter(pw, hasher), limited, make([]byte, copyBufSize))
		sum := make([]byte, sha256.Size)
		_, readErr := io.ReadFull(a.plain, sum)
		if copyErr != nil {
			_ = pw.CloseWithError(copyErr)
			return
		}
		if readErr != nil {
			_ = pw.CloseWithError(readErr)
			return
		}
		if !bytes.Equal(sum, hasher.Sum(nil)) {
			_ = pw.CloseWithError(fmt.Errorf("component %s checksum mismatch", name))
			return
		}
		_ = pw.Close()
	}()
	return name, pr, nil
}

func (a *ArchiveReader) LegacyPayload() []byte {
	if a == nil {
		return nil
	}
	return a.legacy
}

func (a *ArchiveReader) Close() error {
	var first error
	for _, c := range a.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func ObjectKey(acc *store.Account, createdAt string) string {
	if acc == nil {
		return createdAt + ".hpm"
	}
	return fmt.Sprintf("%s/%s.hpm", acc.Username, createdAt)
}
