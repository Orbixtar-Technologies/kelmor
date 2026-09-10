package operations

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxManagedReadBytes = 2 << 20

func (h *Host) readManagedFile(path string) (Result, error) {
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return Result{}, fmt.Errorf("file not found")
	}
	if info.IsDir() {
		return Result{}, fmt.Errorf("path is a directory")
	}
	if info.Size() > maxManagedReadBytes {
		return Result{}, fmt.Errorf("file exceeds read limit")
	}
	body, err := os.ReadFile(p)
	if err != nil {
		return Result{}, err
	}
	return Result{
		OK:            true,
		ObservedState: "read",
		Message:       base64.StdEncoding.EncodeToString(body),
	}, nil
}

func (h *Host) deleteManagedFile(path string) (Result, error) {
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return Result{}, fmt.Errorf("path not found")
	}
	if info.IsDir() {
		entries, err := os.ReadDir(p)
		if err != nil {
			return Result{}, err
		}
		if len(entries) > 0 {
			return Result{}, fmt.Errorf("directory is not empty")
		}
		if err := os.Remove(p); err != nil {
			return Result{}, err
		}
		return Result{OK: true, ObservedState: "absent"}, nil
	}
	if err := os.Remove(p); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "absent"}, nil
}

func (h *Host) renameManagedPath(oldPath, newPath string) (Result, error) {
	oldAbs, err := h.resolve(oldPath)
	if err != nil {
		return Result{}, err
	}
	newAbs, err := h.resolve(newPath)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(oldAbs); err != nil {
		return Result{}, fmt.Errorf("source not found")
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o750); err != nil {
		return Result{}, err
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return Result{}, err
	}
	h.chownAccountPath(newAbs)
	return Result{OK: true, ObservedState: "renamed"}, nil
}

func (h *Host) chmodManagedPath(path string, mode uint32) (Result, error) {
	if mode == 0 {
		return Result{}, fmt.Errorf("mode required")
	}
	p, err := h.resolve(path)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(p); err != nil {
		return Result{}, fmt.Errorf("path not found")
	}
	if err := os.Chmod(p, os.FileMode(mode)); err != nil {
		return Result{}, err
	}
	return Result{OK: true, ObservedState: "chmod"}, nil
}

func decodeManagedContent(content, contentB64 string) ([]byte, error) {
	if contentB64 != "" {
		return base64.StdEncoding.DecodeString(contentB64)
	}
	return []byte(content), nil
}

func accountHomePrefix(username string) string {
	return strings.TrimSuffix(filepath.Join("/home", username), "/")
}
