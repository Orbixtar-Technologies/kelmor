package backup

import (
	"os"
	"path/filepath"
	"strings"
)

func safe(root, key string) (string, error) {
	clean := filepath.Clean("/" + key)
	full := filepath.Join(root, strings.TrimPrefix(clean, "/"))
	if !strings.HasPrefix(full, filepath.Clean(root)+string(os.PathSeparator)) && full != filepath.Clean(root) {
		return "", os.ErrPermission
	}
	return full, nil
}

func writeAtomic(root, key string, data []byte) error {
	p, err := safe(root, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	tmp := p + ".staging"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func readPath(root, key string) ([]byte, error) {
	p, err := safe(root, key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func removePath(root, key string) error {
	p, err := safe(root, key)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func listPrefix(root, prefix string) ([]Object, error) {
	base, err := safe(root, prefix)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Object
	for _, e := range entries {
		info, _ := e.Info()
		sz := int64(0)
		if info != nil {
			sz = info.Size()
		}
		out = append(out, Object{Key: filepath.Join(prefix, e.Name()), Size: sz})
	}
	return out, nil
}
