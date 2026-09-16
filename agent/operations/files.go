package operations

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const maxManagedReadBytes = 2 << 20

func (h *Host) readManagedFile(path string) (Result, error) {
	body, err := h.readManaged(path, maxManagedReadBytes)
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
	root, rel, err := h.openManaged(path)
	if err != nil {
		return Result{}, err
	}
	defer root.Close()
	info, err := root.Stat(rel)
	if err != nil {
		return Result{}, fmt.Errorf("path not found")
	}
	if info.IsDir() {
		dir, err := root.OpenFile(rel, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if err != nil {
			return Result{}, err
		}
		entries, err := dir.ReadDir(-1)
		_ = dir.Close()
		if err != nil {
			return Result{}, err
		}
		if len(entries) > 0 {
			return Result{}, fmt.Errorf("directory is not empty")
		}
	}
	if err := root.Remove(rel); err != nil {
		if info.IsDir() {
			if err := root.RemoveTree(rel); err != nil {
				return Result{}, err
			}
		} else {
			return Result{}, err
		}
	}
	return Result{OK: true, ObservedState: "absent"}, nil
}

func (h *Host) renameManagedPath(oldPath, newPath string) (Result, error) {
	oldRoot, oldRel, err := h.openManaged(oldPath)
	if err != nil {
		return Result{}, err
	}
	defer oldRoot.Close()
	newRoot, newRel, err := h.openManaged(newPath)
	if err != nil {
		return Result{}, err
	}
	defer newRoot.Close()
	if oldRoot.Path() != newRoot.Path() {
		return Result{}, fmt.Errorf("rename across managed roots")
	}
	if _, err := oldRoot.Stat(oldRel); err != nil {
		return Result{}, fmt.Errorf("source not found")
	}
	if err := oldRoot.Rename(oldRel, newRel); err != nil {
		return Result{}, err
	}
	if resolved, err := h.resolve(newPath); err == nil {
		h.chownAccountPath(resolved)
	}
	return Result{OK: true, ObservedState: "renamed"}, nil
}

func (h *Host) chmodManagedPath(path string, mode uint32) (Result, error) {
	if mode == 0 {
		return Result{}, fmt.Errorf("mode required")
	}
	root, rel, err := h.openManaged(path)
	if err != nil {
		return Result{}, err
	}
	defer root.Close()
	if err := root.Chmod(rel, mode); err != nil {
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
