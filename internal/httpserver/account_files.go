package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/agent/policy"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

func (a *API) registerFileRoutes(r chi.Router) {
	r.Get("/files/content", a.readFileContent)
	r.Post("/files/mkdir", a.mkdirPath)
	r.Delete("/files", a.deleteFile)
	r.Patch("/files", a.patchFile)
}

func (a *API) resolveAccountFile(w http.ResponseWriter, r *http.Request, aid, rel, capability string) (string, *store.Account, bool) {
	if !a.requireAccount(w, r, aid, capability) {
		return "", nil, false
	}
	acc := a.Store.GetAccount(aid)
	if acc == nil {
		a.fail(w, r, 404, "NOT_FOUND", "account missing", false)
		return "", nil, false
	}
	if rel == "" {
		rel = "/"
	}
	abs := filepath.Join(acc.HomePath, strings.TrimPrefix(rel, "/"))
	clean, err := policy.WithinAccount(acc.Username, filepath.Clean(abs))
	if err != nil {
		a.fail(w, r, 400, "PATH_DENIED", err.Error(), false)
		return "", nil, false
	}
	return clean, acc, true
}

func (a *API) readFileContent(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	clean, _, ok := a.resolveAccountFile(w, r, aid, r.URL.Query().Get("path"), rbac.FilesRead)
	if !ok {
		return
	}
	params, _ := json.Marshal(map[string]any{"path": clean})
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadManagedFile",
		Params: params,
	})
	if err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	b, _ := json.Marshal(raw)
	var result struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(b, &result)
	content, err := base64.StdEncoding.DecodeString(result.Message)
	if err != nil {
		a.fail(w, r, 500, "FILE_ERROR", "could not decode file content", false)
		return
	}
	writeJSON(w, 200, map[string]any{
		"path":    r.URL.Query().Get("path"),
		"content": string(content),
		"size":    len(content),
	})
}

func (a *API) deleteFile(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	var in struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	clean, _, ok := a.resolveAccountFile(w, r, aid, in.Path, rbac.FilesWrite)
	if !ok {
		return
	}
	params, _ := json.Marshal(map[string]any{"path": clean})
	if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "DeleteManagedFile",
		Params: params,
	}); err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "files.delete", "account", aid, true, map[string]any{"path": in.Path}, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) patchFile(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	var in struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
		Mode    string `json:"mode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.NewPath != "" {
		oldClean, _, ok := a.resolveAccountFile(w, r, aid, in.Path, rbac.FilesWrite)
		if !ok {
			return
		}
		newClean, _, ok := a.resolveAccountFile(w, r, aid, in.NewPath, rbac.FilesWrite)
		if !ok {
			return
		}
		params, _ := json.Marshal(map[string]any{"old_path": oldClean, "new_path": newClean})
		if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "RenameManagedPath",
			Params: params,
		}); err != nil {
			a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
			return
		}
		a.audit(r, "files.rename", "account", aid, true, map[string]any{"path": in.Path, "new_path": in.NewPath}, nil)
		writeJSON(w, 200, map[string]any{"ok": true, "path": in.NewPath})
		return
	}
	if in.Mode != "" {
		mode, err := parseFileMode(in.Mode)
		if err != nil {
			a.fail(w, r, 400, "VALIDATION", err.Error(), false)
			return
		}
		clean, _, ok := a.resolveAccountFile(w, r, aid, in.Path, rbac.FilesWrite)
		if !ok {
			return
		}
		params, _ := json.Marshal(map[string]any{"path": clean, "mode": mode})
		if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "ChmodManagedPath",
			Params: params,
		}); err != nil {
			a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
			return
		}
		a.audit(r, "files.chmod", "account", aid, true, map[string]any{"path": in.Path, "mode": in.Mode}, nil)
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	a.fail(w, r, 400, "VALIDATION", "new_path or mode required", false)
}

func (a *API) mkdirPath(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "accountID")
	var in struct {
		Path string `json:"path"`
		Mode uint32 `json:"mode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	clean, _, ok := a.resolveAccountFile(w, r, aid, in.Path, rbac.FilesWrite)
	if !ok {
		return
	}
	if in.Mode == 0 {
		in.Mode = 0o755
	}
	params, _ := json.Marshal(map[string]any{"path": clean, "mode": in.Mode})
	if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "CreateDirectoryTree",
		Params: params,
	}); err != nil {
		a.fail(w, r, 400, "FILE_ERROR", err.Error(), false)
		return
	}
	a.audit(r, "files.mkdir", "account", aid, true, map[string]any{"path": in.Path}, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func parseFileMode(raw string) (uint32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("mode must be octal like 0644")
	}
	if strings.HasPrefix(raw, "0") {
		v, err := strconv.ParseUint(raw, 8, 32)
		if err != nil {
			return 0, errors.New("mode must be octal like 0644")
		}
		return uint32(v), nil
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, errors.New("mode must be octal like 0644")
	}
	return uint32(v), nil
}
