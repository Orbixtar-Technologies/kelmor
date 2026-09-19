package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hosting-panel/panel/internal/hostconfig"
	"github.com/hosting-panel/panel/internal/rbac"
)

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

func (a *API) loadDirectorSettings() (directorSettingsFile, error) {
	loaded, err := hostconfig.Load(a.panelStateDir())
	return directorSettingsFile{Values: loaded.Values}, err
}

func (a *API) storeDirectorSettings(next directorSettingsFile) error {
	return hostconfig.Store(a.panelStateDir(), hostconfig.File{Values: next.Values})
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

	keys := settingKeys(cleaned)
	directorSettingsMu.Lock()
	current, err := a.loadDirectorSettings()
	if err != nil {
		directorSettingsMu.Unlock()
		a.fail(w, r, 500, "SETTINGS_READ_ERROR", "Could not read Director settings", false)
		return
	}
	if current.Values == nil {
		current.Values = map[string]map[string]string{}
	}
	if row := cleaned["ip_pool"]; row != nil {
		if addr := row["address"]; addr != "" {
			prev := ""
			if current.Values["ip_pool"] != nil {
				prev = current.Values["ip_pool"]["address"]
			}
			row["address"] = mergeSettingLines(prev, addr)
		}
	}
	if row := cleaned["ip_pool_removed"]; row != nil {
		if gone := row["address"]; gone != "" && current.Values["ip_pool"] != nil {
			current.Values["ip_pool"]["address"] = removeSettingLine(current.Values["ip_pool"]["address"], gone)
		}
	}
	for key, fields := range cleaned {
		current.Values[key] = fields
	}
	if err := a.storeDirectorSettings(current); err != nil {
		directorSettingsMu.Unlock()
		a.fail(w, r, 500, "SETTINGS_WRITE_ERROR", "Could not persist Director settings: "+err.Error(), false)
		return
	}
	directorSettingsMu.Unlock()
	a.audit(r, "server.settings.update", "server", "director-settings", true, nil, map[string]any{"keys": keys})
	out := map[string]any{"values": current.Values}
	if hostconfig.NeedsHostApply(keys) {
		job, err := a.enqueueHostConfigJob(r, keys)
		if err != nil {
			a.fail(w, r, 500, "SETTINGS_APPLY_ERROR", "Settings saved but host apply could not be queued: "+err.Error(), false)
			return
		}
		out["operation_id"] = job.ID
		writeJSON(w, 202, out)
		return
	}
	writeJSON(w, 200, out)
}

func mergeSettingLines(existing, added string) string {
	seen := map[string]bool{}
	var out []string
	for _, part := range append(strings.Fields(strings.ReplaceAll(existing, ",", " ")), strings.Fields(strings.ReplaceAll(added, ",", " "))...) {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return strings.Join(out, "\n")
}

func removeSettingLine(existing, gone string) string {
	drop := map[string]bool{}
	for _, part := range strings.Fields(strings.ReplaceAll(gone, ",", " ")) {
		if part = strings.TrimSpace(part); part != "" {
			drop[part] = true
		}
	}
	var out []string
	for _, part := range strings.Fields(strings.ReplaceAll(existing, ",", " ")) {
		part = strings.TrimSpace(part)
		if part == "" || drop[part] {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "\n")
}

func settingKeys(values map[string]map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
