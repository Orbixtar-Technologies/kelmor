package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hosting-panel/panel/internal/update"
)

const (
	panelUpdateConfigPath = "/etc/panel/update.env"
	panelUpdateStatusPath = "/var/lib/panel/update-status.json"
	defaultInstallRoot    = "/usr/local/panel"
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
	case "check":
		if request.Automatic != nil {
			return Result{}, fmt.Errorf("automatic is only valid for settings")
		}
		return h.startPanelUpdate("check")
	case "install":
		if request.Automatic != nil {
			return Result{}, fmt.Errorf("automatic is only valid for settings")
		}
		return h.startPanelUpdate("install")
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

func panelUpdateStartArgs(action string) ([]string, error) {
	switch action {
	case "check":
		return []string{"start", "--no-block", "panel-update@check.service"}, nil
	case "install":
		return []string{"start", "--no-block", "panel-update@install.service"}, nil
	default:
		return nil, fmt.Errorf("unsupported panel update action %q", action)
	}
}

func (h *Host) startPanelUpdate(action string) (Result, error) {
	args, err := panelUpdateStartArgs(action)
	if err != nil {
		return Result{}, err
	}
	if h.live() {
		if output, err := runFixed("/bin/systemctl", args...); err != nil {
			return Result{}, fmt.Errorf("start update service: %s", strings.TrimSpace(string(output)))
		}
	}
	return Result{
		OK:            true,
		Message:       action + " requested through " + args[len(args)-1],
		ObservedState: action + "-requested",
	}, nil
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
	configPath, err := h.resolve(panelUpdateConfigPath)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read update settings: %w", err)
	}
	configuredRoot, err := panelUpdateInstallRoot(content)
	if err != nil {
		return err
	}
	installRoot, err := h.resolvePanelInstallRoot(configuredRoot)
	if err != nil {
		return err
	}
	return update.WithInstallOperationLock(installRoot, func() error {
		return h.writeAutomaticUpdateSettingLocked(automatic, configuredRoot)
	})
}

func (h *Host) writeAutomaticUpdateSettingLocked(automatic bool, lockedInstallRoot string) error {
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
	metadata, err := updateFileMetadataFromInfo(info)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read update settings: %w", err)
	}
	currentInstallRoot, err := panelUpdateInstallRoot(content)
	if err != nil {
		return err
	}
	if currentInstallRoot != lockedInstallRoot {
		return fmt.Errorf("update install root changed while acquiring operation lock")
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

	statusPath, err := h.resolve(panelUpdateStatusPath)
	if err != nil {
		return err
	}
	status, statusMetadata, err := readPanelUpdateStatus(statusPath)
	if err != nil {
		return err
	}
	status.Automatic = automatic
	statusContent, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("encode update status: %w", err)
	}
	statusContent = append(statusContent, '\n')

	if err := writeUpdateFileAtomic(path, []byte(strings.Join(lines, "\n")), metadata); err != nil {
		return fmt.Errorf("write update settings: %w", err)
	}
	if err := writeUpdateFileAtomic(statusPath, statusContent, statusMetadata); err != nil {
		return fmt.Errorf("write update status: %w", err)
	}
	return update.ReconcileStatusPermissions(statusPath)
}

func panelUpdateInstallRoot(content []byte) (string, error) {
	installRoot := defaultInstallRoot
	found := false
	for lineNumber, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return "", fmt.Errorf("invalid update config line %d", lineNumber+1)
		}
		if strings.TrimSpace(key) != "PANEL_UPDATE_INSTALL_ROOT" {
			continue
		}
		if found {
			return "", fmt.Errorf("duplicate PANEL_UPDATE_INSTALL_ROOT setting")
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if value != "" {
			installRoot = value
		}
		found = true
	}
	if !filepath.IsAbs(installRoot) || strings.ContainsRune(installRoot, 0) {
		return "", fmt.Errorf("update install root must be absolute")
	}
	return filepath.Clean(installRoot), nil
}

func (h *Host) resolvePanelInstallRoot(installRoot string) (string, error) {
	if !filepath.IsAbs(installRoot) || strings.ContainsRune(installRoot, 0) {
		return "", fmt.Errorf("update install root must be absolute")
	}
	clean := filepath.Clean(installRoot)
	if h.Root == "" {
		return clean, nil
	}
	return filepath.Join(h.Root, strings.TrimPrefix(clean, "/")), nil
}

type updateFileMetadata struct {
	mode os.FileMode
	uid  int
	gid  int
}

func updateFileMetadataFromInfo(info os.FileInfo) (updateFileMetadata, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return updateFileMetadata{}, fmt.Errorf("unsupported update file metadata")
	}
	return updateFileMetadata{
		mode: info.Mode().Perm(),
		uid:  int(stat.Uid),
		gid:  int(stat.Gid),
	}, nil
}

func readPanelUpdateStatus(path string) (update.Status, updateFileMetadata, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return update.Status{}, updateFileMetadata{}, fmt.Errorf("read update status: %w", err)
		}
		return update.Status{State: "unknown"}, updateFileMetadata{
			mode: 0o600,
			uid:  os.Geteuid(),
			gid:  os.Getegid(),
		}, nil
	}
	if !info.Mode().IsRegular() {
		return update.Status{}, updateFileMetadata{}, fmt.Errorf("update status must be a regular file")
	}
	metadata, err := updateFileMetadataFromInfo(info)
	if err != nil {
		return update.Status{}, updateFileMetadata{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return update.Status{}, updateFileMetadata{}, fmt.Errorf("read update status: %w", err)
	}
	var status update.Status
	if err := json.Unmarshal(content, &status); err != nil {
		return update.Status{}, updateFileMetadata{}, fmt.Errorf("decode update status: %w", err)
	}
	return status, metadata, nil
}

func writeUpdateFileAtomic(path string, content []byte, metadata updateFileMetadata) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chown(metadata.uid, metadata.gid); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(metadata.mode); err != nil {
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
