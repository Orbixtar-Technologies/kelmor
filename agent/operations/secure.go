package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/filesystem"
	"github.com/hosting-panel/panel/internal/pkg/validate"
	"golang.org/x/sys/unix"
)

func (h *Host) openManaged(path string) (*filesystem.Root, string, error) {
	clean, err := policy.ValidateManagedPath(path)
	if err != nil {
		return nil, "", err
	}
	if h.Root != "" {
		if err := filesystem.Create(h.Root, "agent sandbox"); err != nil {
			return nil, "", err
		}
		root, err := filesystem.Open(h.Root, "agent sandbox")
		if err != nil {
			return nil, "", err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(clean), "/")
		if !filesystem.ValidRelative(rel) {
			_ = root.Close()
			return nil, "", fmt.Errorf("invalid relative path %q", rel)
		}
		return root, rel, nil
	}
	if username, rest, ok := policy.AccountRelative(clean); ok {
		if err := validate.Username(username); err != nil {
			return nil, "", err
		}
		root, err := openAccountRoot(policy.AccountRoot(username))
		if err != nil {
			return nil, "", err
		}
		if rest == "" {
			_ = root.Close()
			return nil, "", fmt.Errorf("account-relative path required")
		}
		return root, rest, nil
	}
	prefix, rel, err := policy.SplitManaged(clean)
	if err != nil {
		return nil, "", err
	}
	if err := filesystem.Create(prefix, "managed root"); err != nil {
		return nil, "", err
	}
	root, err := filesystem.Open(prefix, "managed root")
	if err != nil {
		return nil, "", err
	}
	return root, rel, nil
}

func openAccountRoot(home string) (*filesystem.Root, error) {
	if err := filesystem.Create(home, "account root"); err != nil {
		return nil, err
	}
	// Quota placement bind-mounts /var/lib/panel/homes/<user> onto
	// /home/<user>. Opening that child from /home with RESOLVE_NO_XDEV
	// fails with EXDEV ("account root: invalid cross-device link").
	// Open the account home from / instead; symlink and magic-link
	// rejection still apply at the home path.
	return filesystem.Open(home, "account root")
}

func (h *Host) writeManaged(path string, content []byte, mode uint32) error {
	if mode == 0 {
		mode = 0o640
	}
	root, rel, err := h.openManaged(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.WriteAtomic(rel, content, mode)
}

func (h *Host) readManaged(path string, limit int64) ([]byte, error) {
	root, rel, err := h.openManaged(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(rel, limit)
}

func (h *Host) mkdirManaged(path string, mode uint32) error {
	if mode == 0 {
		mode = 0o750
	}
	root, rel, err := h.openManaged(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.MkdirAll(rel, mode)
}

func (h *Host) appendManagedChunk(path string, data []byte, mode uint32, offset int64, last bool) error {
	if mode == 0 {
		mode = 0o640
	}
	root, rel, err := h.openManaged(path)
	if err != nil {
		return err
	}
	defer root.Close()
	parent, name, err := root.Parent(rel, true, 0o750)
	if err != nil {
		return err
	}
	defer parent.Close()
	staging := "." + name + ".staging"
	flags := unix.O_WRONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if offset == 0 {
		_ = unix.Unlinkat(int(parent.Fd()), staging, 0)
		flags |= unix.O_CREAT | unix.O_EXCL
	}
	fd, err := unix.Openat(int(parent.Fd()), staging, flags, mode)
	if err != nil {
		return err
	}
	if err := unix.Fchmod(fd, mode); err != nil {
		_ = unix.Close(fd)
		return err
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		_ = unix.Close(fd)
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = unix.Close(fd)
		return fmt.Errorf("staging path is not a regular file")
	}
	if int64(st.Size) != offset {
		_ = unix.Close(fd)
		return fmt.Errorf("file chunk offset mismatch")
	}
	file := os.NewFile(uintptr(fd), staging)
	if _, err := file.Seek(offset, 0); err != nil {
		_ = file.Close()
		return err
	}
	_, err = file.Write(data)
	if syncErr := file.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if !last {
		return nil
	}
	if err := unix.Renameat(int(parent.Fd()), staging, int(parent.Fd()), name); err != nil {
		_ = unix.Unlinkat(int(parent.Fd()), staging, 0)
		return err
	}
	return parent.Sync()
}
