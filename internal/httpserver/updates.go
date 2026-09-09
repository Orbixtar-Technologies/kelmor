package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/update"
)

const panelUpdateStatusPath = "/var/lib/panel/update-status.json"

func (a *API) updateStatus(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	statusPath := panelUpdateStatusPath
	if a.Agent != nil && a.Agent.Sock == "" && a.Agent.Root != "" {
		statusPath = filepath.Join(a.Agent.Root, "var/lib/panel/update-status.json")
	}
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		a.fail(w, r, http.StatusInternalServerError, "UPDATE_STATUS_ERROR", "Could not read update status", false)
		return
	}
	var status update.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		a.fail(w, r, http.StatusInternalServerError, "UPDATE_STATUS_ERROR", "Could not decode update status", false)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *API) checkUpdate(w http.ResponseWriter, r *http.Request) {
	a.mutateUpdate(w, r, "check", nil, http.StatusAccepted)
}

func (a *API) installUpdate(w http.ResponseWriter, r *http.Request) {
	a.mutateUpdate(w, r, "install", nil, http.StatusAccepted)
}

func (a *API) updateSettings(w http.ResponseWriter, r *http.Request) {
	if !a.requireUpdateMutation(w, r) {
		return
	}
	var request struct {
		Automatic *bool `json:"automatic"`
	}
	if err := decodeStrictJSON(r.Body, &request, false); err != nil || request.Automatic == nil {
		a.fail(w, r, http.StatusBadRequest, "INVALID_JSON", "Expected only an automatic boolean", false)
		return
	}
	a.runUpdateMutation(w, r, operations.PanelUpdateRequest{
		Action:    "settings",
		Automatic: request.Automatic,
	}, http.StatusOK, map[string]any{"automatic": *request.Automatic})
}

func (a *API) mutateUpdate(w http.ResponseWriter, r *http.Request, action string, automatic *bool, status int) {
	if !a.requireUpdateMutation(w, r) {
		return
	}
	var empty struct{}
	if err := decodeStrictJSON(r.Body, &empty, true); err != nil {
		a.fail(w, r, http.StatusBadRequest, "INVALID_JSON", "Update action does not accept input", false)
		return
	}
	a.runUpdateMutation(w, r, operations.PanelUpdateRequest{
		Action:    action,
		Automatic: automatic,
	}, status, map[string]any{"action": action})
}

func (a *API) requireUpdateMutation(w http.ResponseWriter, r *http.Request) bool {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return false
	}
	if !actor(r).IsServerScope {
		a.fail(w, r, http.StatusForbidden, "FORBIDDEN", "Server scope required", false)
		return false
	}
	return true
}

func (a *API) runUpdateMutation(
	w http.ResponseWriter,
	r *http.Request,
	request operations.PanelUpdateRequest,
	status int,
	auditAfter map[string]any,
) {
	if a.Agent == nil {
		a.fail(w, r, http.StatusInternalServerError, "AGENT_ERROR", "Update agent is unavailable", false)
		return
	}
	result, err := a.Agent.ManagePanelUpdate(r.Context(), request)
	if err != nil {
		a.fail(w, r, http.StatusInternalServerError, "AGENT_ERROR", "Could not manage server update", false)
		return
	}
	a.audit(r, "server.update."+request.Action, "server_update", "", true, nil, auditAfter)
	writeJSON(w, status, result)
}

func decodeStrictJSON(body io.Reader, destination any, allowEmpty bool) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if allowEmpty && err == io.EOF {
			return nil
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
