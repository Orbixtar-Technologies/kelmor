package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	return os.Rename(tmpPath, path)
}
