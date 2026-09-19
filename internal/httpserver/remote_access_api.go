package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

const (
	remoteAccessKeyPrefix = "hp_remote_"
	remoteAccessHostPath  = "/etc/panel/remote-access-key"
	remoteAccessActorID   = "remote-access-key"
)

func (a *API) getRemoteAccessKey(w http.ResponseWriter, r *http.Request) {
	if !a.requireServer(w, r, rbac.APITokensRead) {
		return
	}
	rec, err := a.loadRemoteAccessKey(r.Context())
	if err != nil || rec == nil {
		writeJSON(w, 200, map[string]any{
			"applied": false, "revoked": false, "host_path": remoteAccessHostPath,
		})
		return
	}
	writeJSON(w, 200, map[string]any{
		"applied":    !rec.Revoked,
		"revoked":    rec.Revoked,
		"prefix":     rec.Prefix,
		"created_at": rec.CreatedAt,
		"host_path":  remoteAccessHostPath,
	})
}

func (a *API) issueRemoteAccessKey(w http.ResponseWriter, r *http.Request) {
	if !a.requireServer(w, r, rbac.APITokensWrite) {
		return
	}
	plain, _, err := auth.NewOpaqueToken()
	if err != nil {
		a.fail(w, r, 500, "REMOTE_ACCESS_KEY_ERROR", "Could not generate remote access key", true)
		return
	}
	key := remoteAccessKeyPrefix + plain
	rec := operations.RemoteAccessRecord{
		Prefix:    auth.TokenPrefix(key),
		Hash:      hex.EncodeToString(auth.HashToken(key)),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Revoked:   false,
	}
	job, err := a.applyRemoteAccessRecord(r, rec, "server.remote_access.issue")
	if err != nil {
		a.fail(w, r, 500, "REMOTE_ACCESS_KEY_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 202, map[string]any{
		"operation_id": job.ID,
		"key":          key,
		"prefix":       rec.Prefix,
		"created_at":   rec.CreatedAt,
		"host_path":    remoteAccessHostPath,
	})
}

func (a *API) revokeRemoteAccessKey(w http.ResponseWriter, r *http.Request) {
	if !a.requireServer(w, r, rbac.APITokensWrite) {
		return
	}
	rec, err := a.loadRemoteAccessKey(r.Context())
	if err != nil || rec == nil {
		a.fail(w, r, 404, "NOT_FOUND", "remote access key missing", false)
		return
	}
	rec.Revoked = true
	job, err := a.applyRemoteAccessRecord(r, *rec, "server.remote_access.revoke")
	if err != nil {
		a.fail(w, r, 500, "REMOTE_ACCESS_KEY_ERROR", err.Error(), false)
		return
	}
	writeJSON(w, 202, map[string]any{"operation_id": job.ID, "revoked": true})
}

func (a *API) applyRemoteAccessRecord(r *http.Request, rec operations.RemoteAccessRecord, action string) (*store.Job, error) {
	if a.Agent != nil && a.Agent.Sock == "" {
		if _, err := a.Agent.Dispatch(r.Context(), operations.Request{
			Method: "WriteRemoteAccessKey",
			Params: mustJSON(rec),
		}); err != nil {
			return nil, err
		}
	}
	return a.enqueueTypedJob(r, &store.Job{
		Type: "host.remote_access.apply", ResourceType: "server",
		Payload: map[string]any{
			"prefix": rec.Prefix, "hash": rec.Hash, "created_at": rec.CreatedAt,
			"revoked": rec.Revoked, "target": "remote-access-key",
		},
	}, a.auditEvent(r, "", action, "server", "", nil, map[string]any{
		"prefix": rec.Prefix, "revoked": rec.Revoked,
	}))
}

func (a *API) loadRemoteAccessKey(ctx context.Context) (*operations.RemoteAccessRecord, error) {
	raw, err := a.Agent.Dispatch(ctx, operations.Request{Method: "ReadRemoteAccessKey"})
	if err != nil {
		return nil, err
	}
	switch rec := raw.(type) {
	case operations.RemoteAccessRecord:
		cp := rec
		return &cp, nil
	case *operations.RemoteAccessRecord:
		return rec, nil
	case map[string]any:
		body, _ := json.Marshal(rec)
		var parsed operations.RemoteAccessRecord
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		return &parsed, nil
	default:
		return nil, nil
	}
}

func (a *API) authenticateRemoteAccessKey(token string) (rbac.Actor, bool) {
	if !strings.HasPrefix(token, remoteAccessKeyPrefix) {
		return rbac.Actor{}, false
	}
	rec, err := a.loadRemoteAccessKey(context.Background())
	if err != nil || rec == nil || rec.Revoked || rec.Hash == "" {
		return rbac.Actor{}, false
	}
	want, err := hex.DecodeString(rec.Hash)
	if err != nil {
		return rbac.Actor{}, false
	}
	got := auth.HashToken(token)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return rbac.Actor{}, false
	}
	return rbac.Actor{
		UserID:        remoteAccessActorID,
		Username:      "remote-access",
		Roles:         []string{"server_administrator"},
		Capabilities:  rbac.Expand([]string{"server_administrator"}, nil),
		IsServerScope: true,
	}, true
}
