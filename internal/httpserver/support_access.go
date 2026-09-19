package httpserver

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/rbac"
	"github.com/hosting-panel/panel/internal/store"
)

const supportAccessUser = "kelmor-support"

var (
	supportGrantMu  sync.Mutex
	supportTicketRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

type supportGrant struct {
	ID        string    `json:"id"`
	Ticket    string    `json:"ticket"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type supportGrantFile struct {
	Items []supportGrant `json:"items"`
}

func (a *API) supportGrantPath() string {
	return filepath.Join(a.panelStateDir(), "control", "support-grants.json")
}

func (a *API) loadSupportGrants() (supportGrantFile, error) {
	out := supportGrantFile{Items: []supportGrant{}}
	raw, err := os.ReadFile(a.supportGrantPath())
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
		return supportGrantFile{}, err
	}
	if out.Items == nil {
		out.Items = []supportGrant{}
	}
	return out, nil
}

func (a *API) storeSupportGrants(next supportGrantFile) error {
	if err := os.MkdirAll(filepath.Dir(a.supportGrantPath()), 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.supportGrantPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.supportGrantPath())
}

func (a *API) listSupportAccess(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	supportGrantMu.Lock()
	defer supportGrantMu.Unlock()
	file, err := a.loadSupportGrants()
	if err != nil {
		a.fail(w, r, 500, "SUPPORT_READ_ERROR", "Could not read support grants", false)
		return
	}
	now := time.Now().UTC()
	items := make([]map[string]any, 0)
	for _, grant := range file.Items {
		if grant.ExpiresAt.Before(now) {
			continue
		}
		items = append(items, map[string]any{
			"id": grant.ID, "ticket": grant.Ticket, "expires_at": grant.ExpiresAt,
			"created_at": grant.CreatedAt, "username": supportAccessUser,
		})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) grantSupportAccess(w http.ResponseWriter, r *http.Request) {
	if !a.requireServer(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in struct {
		Ticket string `json:"ticket"`
		Hours  int    `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid support access request", false)
		return
	}
	ticket := strings.TrimSpace(in.Ticket)
	if !supportTicketRe.MatchString(ticket) {
		a.fail(w, r, 400, "VALIDATION", "ticket must be 1-64 letters, numbers, dots, underscores, or dashes", false)
		return
	}
	hours := in.Hours
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		a.fail(w, r, 400, "VALIDATION", "hours must be between 1 and 168", false)
		return
	}
	password, err := randomSupportPassword()
	if err != nil {
		a.fail(w, r, 500, "SUPPORT_GRANT_ERROR", "Could not generate support credentials", false)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		a.fail(w, r, 500, "SUPPORT_GRANT_ERROR", "Could not generate support credentials", false)
		return
	}
	user := a.Store.UserByUsername(supportAccessUser)
	if user == nil {
		user = &store.User{
			ID: id.New(), Username: supportAccessUser, Email: "support@localhost",
			DisplayName: "Kelmor Support", Status: "active",
			Roles: []string{"server_operator"}, CreatedAt: time.Now().UTC(),
		}
	}
	user.PasswordHash = hash
	user.Status = "active"
	user.Roles = []string{"server_operator"}
	user.MustChangePassword = false
	a.Store.PutUser(user)

	plain, tokenHash, err := auth.NewOpaqueToken()
	if err != nil {
		a.fail(w, r, 500, "SUPPORT_GRANT_ERROR", "Could not create support session", false)
		return
	}
	expires := time.Now().UTC().Add(time.Duration(hours) * time.Hour)
	sess := &store.Session{
		ID: id.New(), UserID: user.ID, TokenHash: tokenHash, ExpiresAt: expires,
		SourceIP: clientIP(r), UserAgent: r.UserAgent(),
		ImpersonatorID:      actor(r).UserID,
		ImpersonationReason: "support:" + ticket,
	}
	a.Store.PutSession(sess)
	grant := supportGrant{
		ID: id.New(), Ticket: ticket, SessionID: sess.ID, UserID: user.ID,
		ExpiresAt: expires, CreatedAt: time.Now().UTC(),
	}
	supportGrantMu.Lock()
	file, err := a.loadSupportGrants()
	if err != nil {
		supportGrantMu.Unlock()
		a.fail(w, r, 500, "SUPPORT_GRANT_ERROR", "Could not persist support grant", false)
		return
	}
	file.Items = append(file.Items, grant)
	if err := a.storeSupportGrants(file); err != nil {
		supportGrantMu.Unlock()
		a.fail(w, r, 500, "SUPPORT_GRANT_ERROR", "Could not persist support grant", false)
		return
	}
	supportGrantMu.Unlock()
	a.audit(r, "server.support.grant", "support_session", grant.ID, true, nil, map[string]any{
		"ticket": ticket, "hours": hours, "username": supportAccessUser,
	})
	writeJSON(w, 200, map[string]any{
		"id": grant.ID, "token": plain, "username": supportAccessUser,
		"password": password, "expires_at": expires, "ticket": ticket,
		"role": "server_operator",
	})
}

func (a *API) revokeSupportAccess(w http.ResponseWriter, r *http.Request) {
	if !a.requireServer(w, r, rbac.ServerSettingsWrite) {
		return
	}
	grantID := chi.URLParam(r, "grantID")
	supportGrantMu.Lock()
	defer supportGrantMu.Unlock()
	file, err := a.loadSupportGrants()
	if err != nil {
		a.fail(w, r, 500, "SUPPORT_READ_ERROR", "Could not read support grants", false)
		return
	}
	kept := file.Items[:0]
	found := false
	for _, grant := range file.Items {
		if grant.ID == grantID {
			found = true
			a.Store.RevokeSession(grant.SessionID)
			continue
		}
		kept = append(kept, grant)
	}
	if !found {
		a.fail(w, r, 404, "NOT_FOUND", "Support grant not found", false)
		return
	}
	file.Items = kept
	if err := a.storeSupportGrants(file); err != nil {
		a.fail(w, r, 500, "SUPPORT_REVOKE_ERROR", "Could not persist support revoke", false)
		return
	}
	a.audit(r, "server.support.revoke", "support_session", grantID, true, nil, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func randomSupportPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf) + "!x", nil
}
