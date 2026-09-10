package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
)

func (a *API) controlService(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerServicesRestart) {
		return
	}
	name := chi.URLParam(r, "serviceName")
	action := chi.URLParam(r, "action")
	if action == "" {
		action = "restart"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "start", "stop", "restart", "reload":
	default:
		a.fail(w, r, 400, "VALIDATION", "action must be start, stop, restart, or reload", false)
		return
	}
	params, _ := json.Marshal(map[string]any{"name": name, "action": action})
	res, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ControlService",
		Params: params,
	})
	if err != nil {
		a.fail(w, r, 500, "AGENT_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "server.service."+action, "service", name, true, nil, map[string]any{"service": name, "action": action})
	writeJSON(w, 200, res)
}
