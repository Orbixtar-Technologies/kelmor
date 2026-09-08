package backup

import "context"

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
	_, err := l.Get(ctx, key)
	return err
}

var _ Repository = (*Local)(nil)
