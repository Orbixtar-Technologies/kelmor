package update

import (
	"crypto/ed25519"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Apply(bundleDir, installRoot string, pub ed25519.PublicKey) error {
	m, err := Load(filepath.Join(bundleDir, "manifest.json"))
	if err != nil {
		return err
	}
	if err := Verify(m, pub); err != nil {
		return err
	}
	if err := VerifyFileHashes(m, bundleDir); err != nil {
		return err
	}
	relDir := filepath.Join(installRoot, "releases", m.Release)
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		return err
	}
	binDir := filepath.Join(installRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	rbDir := filepath.Join(installRoot, "rollback")
	_ = os.RemoveAll(rbDir)
	if err := os.MkdirAll(rbDir, 0o755); err != nil {
		return err
	}
	for rel := range m.Files {
		if strings.Contains(rel, "..") {
			return fmt.Errorf("illegal path %s", rel)
		}
		src := filepath.Join(bundleDir, rel)
		if err := copyFile(src, filepath.Join(relDir, rel)); err != nil {
			return err
		}
		cur := filepath.Join(binDir, filepath.Base(rel))
		if _, err := os.Stat(cur); err == nil {
			if err := copyFile(cur, filepath.Join(rbDir, filepath.Base(rel))); err != nil {
				return err
			}
		}
		if err := copyFile(src, cur); err != nil {
			_ = Rollback(installRoot)
			return err
		}
	}
	return os.WriteFile(filepath.Join(installRoot, "current-release"), []byte(m.Release+"\n"), 0o644)
}

func Rollback(installRoot string) error {
	rbDir := filepath.Join(installRoot, "rollback")
	binDir := filepath.Join(installRoot, "bin")
	entries, err := os.ReadDir(rbDir)
	if err != nil {
		return fmt.Errorf("no rollback snapshot: %w", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(rbDir, e.Name()), filepath.Join(binDir, e.Name())); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(installRoot, "current-release"), []byte("rolled-back\n"), 0o644)
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".staging"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}
