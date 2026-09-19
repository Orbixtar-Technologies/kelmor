package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) listLanguageModules(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ListLanguageModules",
		Params: mustJSON(map[string]any{"kind": kind}),
	})
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	writeJSON(w, 200, map[string]any{"items": raw})
}

func (a *API) installLanguageModule(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid module install", false)
		return
	}
	if err := operations.ValidateLanguageModuleName(in.Kind, in.Name); err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "host.module.install", ResourceType: "server",
		Payload: map[string]any{
			"kind": in.Kind, "name": in.Name, "target": in.Kind + ":" + in.Name,
		},
		State: "queued",
	}, a.auditEvent(r, "", "server.module.install", "server", "", nil, map[string]any{
		"kind": in.Kind, "name": in.Name,
	}))
	if err != nil {
		a.fail(w, r, 500, "MODULE_INSTALL_ERROR", "Could not queue module installation", true)
		return
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID, "status": "provisioning", "kind": in.Kind, "name": in.Name,
	})
}
