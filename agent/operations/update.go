package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	panelUpdateConfigPath = "/etc/panel/update.env"
	panelUpdateService    = "panel-update.service"
)

type PanelUpdateRequest struct {
	Action    string `json:"action"`
	Automatic *bool  `json:"automatic,omitempty"`
}

func (h *Host) ManagePanelUpdate(ctx context.Context, request PanelUpdateRequest) (Result, error) {
	if h.Sock != "" {
		raw, err := CallUnix(ctx, h.Sock, Request{
			Method: "ManagePanelUpdate",
			Params: mustRaw(request),
		})
		if err != nil {
			return Result{}, err
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return Result{}, err
		}
		var result Result
		if err := json.Unmarshal(encoded, &result); err != nil {
			return Result{}, err
		}
		return result, nil
	}

	switch request.Action {
	case "check", "install":
		if request.Automatic != nil {
			return Result{}, fmt.Errorf("automatic is only valid for settings")
		}
		if h.live() {
			if output, err := runFixed("/bin/systemctl", "start", panelUpdateService); err != nil {
				return Result{}, fmt.Errorf("start %s: %s", panelUpdateService, strings.TrimSpace(string(output)))
			}
		}
		return Result{
			OK:            true,
			Message:       request.Action + " requested through " + panelUpdateService,
			ObservedState: request.Action + "-requested",
		}, nil
	case "settings":
		if request.Automatic == nil {
			return Result{}, fmt.Errorf("automatic is required for settings")
		}
		if err := h.writeAutomaticUpdateSetting(*request.Automatic); err != nil {
			return Result{}, err
		}
		state := "automatic-disabled"
		if *request.Automatic {
			state = "automatic-enabled"
		}
		return Result{OK: true, Message: "automatic update setting changed", ObservedState: state}, nil
	default:
		return Result{}, fmt.Errorf("unsupported panel update action %q", request.Action)
	}
}

func decodePanelUpdateRequest(raw json.RawMessage) (PanelUpdateRequest, error) {
	var request PanelUpdateRequest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return PanelUpdateRequest{}, fmt.Errorf("invalid panel update request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return PanelUpdateRequest{}, fmt.Errorf("invalid panel update request")
	}
	return request, nil
}

func (h *Host) writeAutomaticUpdateSetting(automatic bool) error {
	path, err := h.resolve(panelUpdateConfigPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("read update settings: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("update settings must be a regular file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read update settings: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	replacement := fmt.Sprintf("PANEL_UPDATE_AUTOMATIC=%t", automatic)
	found := false
	for index, line := range lines {
		if !strings.HasPrefix(line, "PANEL_UPDATE_AUTOMATIC=") {
			continue
		}
		if found {
			return fmt.Errorf("duplicate PANEL_UPDATE_AUTOMATIC setting")
		}
		lines[index] = replacement
		found = true
	}
	if !found {
		return fmt.Errorf("PANEL_UPDATE_AUTOMATIC setting is missing")
	}

	return writeUpdateConfigAtomic(path, []byte(strings.Join(lines, "\n")))
}

func writeUpdateConfigAtomic(path string, content []byte) error {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	parent, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}
