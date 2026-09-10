package update

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

type Status struct {
	State            string `json:"state"`
	InstalledRelease string `json:"installed_release"`
	AvailableRelease string `json:"available_release,omitempty"`
	LastCheckedAt    string `json:"last_checked_at,omitempty"`
	Error            string `json:"error,omitempty"`
	Automatic        bool   `json:"automatic"`
	Channel          string `json:"channel"`
}

func WriteStatus(path string, status Status) error {
	if path == "" {
		return fmt.Errorf("status path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	if err := finalizeStatusPermissions(path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func ReconcileStatusPermissions(path string) error {
	return finalizeStatusPermissions(path)
}

func finalizeStatusPermissions(path string) error {
	if err := os.Chmod(path, 0o640); err != nil {
		return err
	}
	panelUser, err := user.Lookup("panel")
	if err != nil {
		return nil
	}
	uid, convErr := strconv.Atoi(panelUser.Uid)
	if convErr != nil {
		return nil
	}
	gid, convErr := strconv.Atoi(panelUser.Gid)
	if convErr != nil {
		return nil
	}
	return os.Chown(path, uid, gid)
}
