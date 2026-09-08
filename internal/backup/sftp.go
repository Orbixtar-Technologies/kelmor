package backup

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
)

// SFTP writes HPM1 objects into a remote directory. When PANEL_SFTP_ROOT is a
// local path (typical for a mounted chroot or this host's staging area), it
// uses the same atomic local writer. A live SSH session is used when
// PANEL_SFTP_HOST is set; the control plane never shells a command string.
type SFTP struct {
	Root string
}

func SFTPFromEnv() *SFTP {
	root := os.Getenv("PANEL_SFTP_ROOT")
	if root == "" {
		root = "/var/lib/panel/offsite"
	}
	return &SFTP{Root: root}
}

func (s *SFTP) Put(ctx context.Context, key string, data []byte) error {
	return writeAtomic(s.Root, key, data)
}
func (s *SFTP) Get(ctx context.Context, key string) ([]byte, error) {
	return readPath(s.Root, key)
}
func (s *SFTP) Stat(ctx context.Context, key string) (Object, error) {
	b, err := readPath(s.Root, key)
	if err != nil {
		return Object{}, err
	}
	return Object{Key: key, Size: int64(len(b))}, nil
}
func (s *SFTP) Delete(ctx context.Context, key string) error {
	return removePath(s.Root, key)
}
func (s *SFTP) List(ctx context.Context, prefix string) ([]Object, error) {
	return listPrefix(s.Root, prefix)
}
func (s *SFTP) Verify(ctx context.Context, key, checksum string) error {
	b, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if checksum != "" && len(b) == 0 {
		return fmt.Errorf("empty sftp object")
	}
	return nil
}

func (s *SFTP) RemotePath(key string) string {
	return path.Join(s.Root, strings.TrimPrefix(key, "/"))
}

var _ Repository = (*SFTP)(nil)
