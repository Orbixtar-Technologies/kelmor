package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

type Object struct {
	Key      string
	Size     int64
	Checksum string
}

type Repository interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Stat(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]Object, error)
	Verify(ctx context.Context, key, checksum string) error
}

type StreamRepository interface {
	Repository
	PutStream(ctx context.Context, key string, r io.Reader) error
	GetStream(ctx context.Context, key string) (io.ReadCloser, error)
}

func putStream(ctx context.Context, repo Repository, key string, r io.Reader) error {
	if s, ok := repo.(interface {
		PutStream(context.Context, string, io.Reader) error
	}); ok {
		return s.PutStream(ctx, key, r)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return repo.Put(ctx, key, data)
}

func getStream(ctx context.Context, repo Repository, key string) (io.ReadCloser, error) {
	if s, ok := repo.(interface {
		GetStream(context.Context, string) (io.ReadCloser, error)
	}); ok {
		return s.GetStream(ctx, key)
	}
	data, err := repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func objectChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func verifyObject(data []byte, checksum string) error {
	if checksum == "" {
		return nil
	}
	if len(data) == 0 {
		return fmt.Errorf("empty backup object")
	}
	if objectChecksum(data) != checksum {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

type Local struct {
	Root string
}

func (l *Local) Put(ctx context.Context, key string, data []byte) error {
	return writeAtomic(l.Root, key, data)
}
func (l *Local) Get(ctx context.Context, key string) ([]byte, error) {
	return readPath(l.Root, key)
}
func (l *Local) Stat(ctx context.Context, key string) (Object, error) {
	b, err := readPath(l.Root, key)
	if err != nil {
		return Object{}, err
	}
	return Object{Key: key, Size: int64(len(b))}, nil
}
func (l *Local) Delete(ctx context.Context, key string) error {
	return removePath(l.Root, key)
}
func (l *Local) List(ctx context.Context, prefix string) ([]Object, error) {
	return listPrefix(l.Root, prefix)
}
func (l *Local) Verify(ctx context.Context, key, checksum string) error {
	b, err := l.Get(ctx, key)
	if err != nil {
		return err
	}
	return verifyObject(b, checksum)
}

func (l *Local) PutStream(ctx context.Context, key string, r io.Reader) error {
	return writeAtomicStream(l.Root, key, r)
}

func (l *Local) GetStream(ctx context.Context, key string) (io.ReadCloser, error) {
	return openPath(l.Root, key)
}

var _ StreamRepository = (*Local)(nil)
