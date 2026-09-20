package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/update"
)

const panelUpdateStatusPath = "/var/lib/panel/update-status.json"

func (a *API) updateStatus(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	status, err := a.readUpdateStatus()
	if err != nil {
		a.fail(w, r, http.StatusInternalServerError, "UPDATE_STATUS_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *API) updateChangelog(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerRead) {
		return
	}
	status, err := a.readUpdateStatus()
	if err != nil {
		a.fail(w, r, http.StatusInternalServerError, "UPDATE_STATUS_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, http.StatusOK, a.changelogView(status))
}

func (a *API) readUpdateStatus() (update.Status, error) {
	raw, err := os.ReadFile(a.updateStatusPath())
	if err != nil {
		return update.Status{}, fmt.Errorf("Could not read update status")
	}
	var status update.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		return update.Status{}, fmt.Errorf("Could not decode update status")
	}
	if a.Version != "" {
		status.RunningRelease = a.Version
		status.InstalledRelease = a.Version
	}
	return status, nil
}

func (a *API) updateStatusPath() string {
	if a.Agent != nil && a.Agent.Sock == "" && a.Agent.Root != "" {
		return filepath.Join(a.Agent.Root, "var/lib/panel/update-status.json")
	}
	return panelUpdateStatusPath
}

func (a *API) updateInstallRoot() string {
	if a.Agent != nil && a.Agent.Sock == "" && a.Agent.Root != "" {
		return a.Agent.Root
	}
	return ""
}

func (a *API) changelogView(status update.Status) update.ChangelogView {
	docs := make([]update.ChangelogDocument, 0, 3)
	if cached, err := update.ReadChangelogFile(
		update.ChangelogCachePath(a.updateStatusPath(), a.updateInstallRoot()),
		"update-feed",
	); err == nil && len(cached.Items) > 0 {
		docs = append(docs, cached)
	}
	if local, err := update.ReadChangelogFile(
		update.FeedChangelogPath(a.updateInstallRoot(), status.Channel),
		"update-feed",
	); err == nil && len(local.Items) > 0 {
		docs = append(docs, local)
	}
	if embedded := update.EmbeddedChangelogDocument(); len(embedded.Items) > 0 {
		docs = append(docs, embedded)
	}
	running := status.RunningRelease
	if running == "" {
		running = a.Version
	}
	return update.BuildChangelogView(status, running, docs...)
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
	if bearer(r) == "" {
		a.fail(w, r, http.StatusUnauthorized, "BEARER_REQUIRED", "Update mutations require bearer authentication", false)
		return false
	}
	if !updateMutationOriginAllowed(r) {
		a.fail(w, r, http.StatusForbidden, "ORIGIN_FORBIDDEN", "Update mutation origin is not allowed", false)
		return false
	}
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return false
	}
	if !actor(r).IsServerScope {
		a.fail(w, r, http.StatusForbidden, "FORBIDDEN", "Server scope required", false)
		return false
	}
	return true
}

func updateMutationOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	requestScheme, requestHost, requestPort, ok := externalRequestAuthority(r)
	if !ok {
		return false
	}
	originScheme, originHost, originPort, ok := normalizedOriginAuthority(origin)
	if !ok {
		return false
	}
	return originScheme == requestScheme &&
		strings.EqualFold(originHost, requestHost) &&
		originPort == requestPort
}

func externalRequestAuthority(r *http.Request) (scheme, hostname, port string, ok bool) {
	scheme = "http"
	authority := r.Host
	if r.TLS != nil {
		scheme = "https"
	} else if forwardedProto, trusted := trustedForwardedProto(r); trusted {
		forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
		if forwardedHost == "" || strings.Contains(forwardedHost, ",") {
			return "", "", "", false
		}
		scheme = forwardedProto
		authority = forwardedHost
	}
	hostname, port, ok = normalizedHostPort(authority, scheme)
	return scheme, hostname, port, ok
}

func normalizedOriginAuthority(origin string) (scheme, hostname, port string, ok bool) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", "", false
	}
	scheme = strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", "", false
	}
	hostname, port, ok = normalizedHostPort(parsed.Host, scheme)
	return scheme, hostname, port, ok
}

func normalizedHostPort(authority, scheme string) (hostname, port string, ok bool) {
	parsed, err := url.Parse("//" + authority)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", false
	}
	port = parsed.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", "", false
		}
	}
	return parsed.Hostname(), port, true
}

func (a *API) runUpdateMutation(
	w http.ResponseWriter,
	r *http.Request,
	request operations.PanelUpdateRequest,
	status int,
	auditAfter map[string]any,
) {
	action := "server.update." + request.Action
	a.audit(r, action+".intent", "server_update", "", true, nil, auditAfter)
	if a.Agent == nil {
		a.fail(w, r, http.StatusInternalServerError, "AGENT_ERROR", "Update agent is unavailable", false)
		return
	}
	result, err := a.Agent.ManagePanelUpdate(r.Context(), request)
	if err != nil {
		a.fail(w, r, http.StatusInternalServerError, "AGENT_ERROR", "Could not manage server update", false)
		return
	}
	a.audit(r, action, "server_update", "", true, nil, auditAfter)
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
