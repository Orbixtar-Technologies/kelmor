package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) getPostgresConfig(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListPostgresConfig"})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, raw)
}

func (a *API) getMySQLUpgrade(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{Method: "ListMySQLUpgrade"})
	if err != nil {
		a.fail(w, r, 400, "AGENT_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 200, raw)
}

func (a *API) queueMySQLUpgrade(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid MariaDB upgrade request", false)
		return
	}
	target := strings.TrimSpace(in.Target)
	if !mysqlUpgradeTargetOK(target) {
		a.fail(w, r, 400, "VALIDATION", "unsupported MariaDB target version", false)
		return
	}
	job, err := a.enqueueTypedJob(r, &store.Job{
		Type: "mysql.upgrade", ResourceType: "server",
		Payload: map[string]any{"target": target},
		State:   "queued",
	}, a.auditEvent(r, "", "server.mysql.upgrade", "server", "", nil, map[string]any{
		"target": target,
	}))
	if err != nil {
		a.fail(w, r, 500, "MYSQL_UPGRADE_ERROR", "Could not queue MariaDB upgrade", true)
		return
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID, "status": "provisioning", "target": target,
	})
}

func mysqlUpgradeTargetOK(target string) bool {
	switch target {
	case "10.11", "11.4":
		return true
	default:
		return false
	}
}
