package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hosting-panel/panel/internal/rbac"
)

const directorSettingsName = "director-settings.json"

var (
	directorSettingsMu   sync.Mutex
	directorSettingKeyRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	secretSettingField   = regexp.MustCompile(`(?i)password|secret|private[_-]?key|passwd`)
)

type directorSettingsFile struct {
	Values map[string]map[string]string `json:"values"`
}

func (a *API) panelStateDir() string {
	base := strings.TrimSpace(os.Getenv("PANEL_STATE_DIR"))
	if base == "" {
		base = "/var/lib/panel"
	}
	if a.Agent != nil && a.Agent.Sock == "" && a.Agent.Root != "" {
		base = filepath.Join(a.Agent.Root, "var/lib/panel")
	}
	return base
}

func (a *API) directorSettingsPath() string {
	return filepath.Join(a.panelStateDir(), directorSettingsName)
}

func (a *API) loadDirectorSettings() (directorSettingsFile, error) {
	out := directorSettingsFile{Values: map[string]map[string]string{}}
	raw, err := os.ReadFile(a.directorSettingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return directorSettingsFile{Values: map[string]map[string]string{}}, err
	}
	if out.Values == nil {
		out.Values = map[string]map[string]string{}
	}
	return out, nil
}

func (a *API) storeDirectorSettings(next directorSettingsFile) error {
	dir := a.panelStateDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.directorSettingsPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.directorSettingsPath())
}

func (a *API) getDirectorSettings(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	directorSettingsMu.Lock()
	defer directorSettingsMu.Unlock()
	settings, err := a.loadDirectorSettings()
	if err != nil {
		a.fail(w, r, 500, "SETTINGS_READ_ERROR", "Could not read Director settings", false)
		return
	}
	writeJSON(w, 200, settings)
}

func (a *API) patchDirectorSettings(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in directorSettingsFile
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid settings", false)
		return
	}
	if in.Values == nil {
		a.fail(w, r, 400, "VALIDATION", "values is required", false)
		return
	}
	cleaned := map[string]map[string]string{}
	for key, fields := range in.Values {
		if !directorSettingKeyRe.MatchString(key) {
			a.fail(w, r, 400, "VALIDATION", "invalid settings key", false)
			return
		}
		safe := map[string]string{}
		for field, value := range fields {
			if !directorSettingKeyRe.MatchString(field) {
				a.fail(w, r, 400, "VALIDATION", "invalid settings field", false)
				return
			}
			if secretSettingField.MatchString(field) {
				a.fail(w, r, 400, "VALIDATION", "passwords and secrets cannot be stored in Director settings", false)
				return
			}
			if len(value) > 8192 {
				a.fail(w, r, 400, "VALIDATION", "settings value too large", false)
				return
			}
			safe[field] = value
		}
		cleaned[key] = safe
	}

	directorSettingsMu.Lock()
	defer directorSettingsMu.Unlock()
	current, err := a.loadDirectorSettings()
	if err != nil {
		a.fail(w, r, 500, "SETTINGS_READ_ERROR", "Could not read Director settings", false)
		return
	}
	if current.Values == nil {
		current.Values = map[string]map[string]string{}
	}
	for key, fields := range cleaned {
		current.Values[key] = fields
	}
	if err := a.storeDirectorSettings(current); err != nil {
		a.fail(w, r, 500, "SETTINGS_WRITE_ERROR", "Could not persist Director settings", false)
		return
	}
	a.audit(r, "server.settings.update", "server", "director-settings", true, nil, map[string]any{"keys": settingKeys(cleaned)})
	writeJSON(w, 200, current)
}

func settingKeys(values map[string]map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
