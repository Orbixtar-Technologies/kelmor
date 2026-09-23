package httpserver

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hosting-panel/panel/internal/rbac"
)

const (
	githubOAuthAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubOAuthTokenURL     = "https://github.com/login/oauth/access_token"
	githubAPIBase           = "https://api.github.com"
	githubOAuthScope        = "repo read:user"
	githubOAuthSecretFile   = "github-oauth.json"
)

type githubOAuthAppConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url,omitempty"`
}

type githubOAuthConnection struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	Login       string    `json:"login"`
	ConnectedAt time.Time `json:"connected_at"`
}

type githubOAuthState struct {
	UserID    string
	ExpiresAt time.Time
}

var (
	githubOAuthMu     sync.Mutex
	githubOAuthStates = map[string]githubOAuthState{}
)

func (a *API) githubSecretsPath() string {
	return filepath.Join(a.panelStateDir(), "secrets", githubOAuthSecretFile)
}

func (a *API) githubConnectionsDir() string {
	return filepath.Join(a.panelStateDir(), "secrets", "github-connections")
}

func (a *API) loadGitHubOAuthApp() (githubOAuthAppConfig, error) {
	var cfg githubOAuthAppConfig
	raw, err := os.ReadFile(a.githubSecretsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (a *API) storeGitHubOAuthApp(cfg githubOAuthAppConfig) error {
	dir := filepath.Dir(a.githubSecretsPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.githubSecretsPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.githubSecretsPath())
}

func (a *API) githubConnectionPath(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return filepath.Join(a.githubConnectionsDir(), hex.EncodeToString(sum[:16])+".json")
}

func (a *API) loadGitHubConnection(userID string) (githubOAuthConnection, error) {
	var conn githubOAuthConnection
	raw, err := os.ReadFile(a.githubConnectionPath(userID))
	if err != nil {
		if os.IsNotExist(err) {
			return conn, nil
		}
		return conn, err
	}
	if err := json.Unmarshal(raw, &conn); err != nil {
		return conn, err
	}
	return conn, nil
}

func (a *API) storeGitHubConnection(userID string, conn githubOAuthConnection) error {
	if err := os.MkdirAll(a.githubConnectionsDir(), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return err
	}
	path := a.githubConnectionPath(userID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *API) deleteGitHubConnection(userID string) {
	_ = os.Remove(a.githubConnectionPath(userID))
}

func (a *API) putGitHubOAuthApp(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ServerSettingsWrite) {
		return
	}
	var in githubOAuthAppConfig
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		a.fail(w, r, 400, "INVALID_JSON", "Invalid GitHub OAuth app", false)
		return
	}
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.ClientSecret = strings.TrimSpace(in.ClientSecret)
	in.RedirectURL = strings.TrimSpace(in.RedirectURL)
	if in.ClientID == "" || in.ClientSecret == "" {
		a.fail(w, r, 400, "VALIDATION", "client_id and client_secret are required", false)
		return
	}
	if err := a.storeGitHubOAuthApp(in); err != nil {
		a.fail(w, r, 500, "STORAGE_ERROR", "Could not store GitHub OAuth app", true)
		return
	}
	a.audit(r, "github.oauth.app.configure", "integration", "github", true, nil, map[string]any{"client_id": in.ClientID})
	writeJSON(w, 200, map[string]any{"ok": true, "client_id": in.ClientID, "configured": true})
}

func (a *API) githubOAuthStatus(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ApplicationsRead) {
		return
	}
	cfg, err := a.loadGitHubOAuthApp()
	if err != nil {
		a.fail(w, r, 500, "STORAGE_ERROR", "Could not read GitHub OAuth config", true)
		return
	}
	conn, _ := a.loadGitHubConnection(actor(r).UserID)
	writeJSON(w, 200, map[string]any{
		"app_configured": cfg.ClientID != "",
		"client_id":      cfg.ClientID,
		"connected":      conn.AccessToken != "",
		"login":          conn.Login,
		"connected_at":   conn.ConnectedAt,
		"scope":          conn.Scope,
	})
}

func (a *API) githubOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ApplicationsWrite) {
		return
	}
	cfg, err := a.loadGitHubOAuthApp()
	if err != nil || cfg.ClientID == "" || cfg.ClientSecret == "" {
		a.fail(w, r, 400, "NOT_CONFIGURED", "GitHub OAuth app is not configured on this Director", false)
		return
	}
	state, err := randomHex(24)
	if err != nil {
		a.fail(w, r, 500, "TOKEN_ERROR", "Could not create OAuth state", true)
		return
	}
	githubOAuthMu.Lock()
	githubOAuthStates[state] = githubOAuthState{UserID: actor(r).UserID, ExpiresAt: time.Now().Add(10 * time.Minute)}
	githubOAuthMu.Unlock()

	redirect := cfg.RedirectURL
	if redirect == "" {
		redirect = a.publicOrigin(r) + "/api/v1/integrations/github/callback"
	}
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("scope", githubOAuthScope)
	q.Set("state", state)
	writeJSON(w, 200, map[string]any{"authorize_url": githubOAuthAuthorizeURL + "?" + q.Encode()})
}

func (a *API) githubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}
	githubOAuthMu.Lock()
	st, ok := githubOAuthStates[state]
	delete(githubOAuthStates, state)
	githubOAuthMu.Unlock()
	if !ok || time.Now().After(st.ExpiresAt) {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	cfg, err := a.loadGitHubOAuthApp()
	if err != nil || cfg.ClientID == "" {
		http.Error(w, "oauth app not configured", http.StatusBadRequest)
		return
	}
	redirect := cfg.RedirectURL
	if redirect == "" {
		redirect = a.publicOrigin(r) + "/api/v1/integrations/github/callback"
	}
	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	req, err := http.NewRequest(http.MethodPost, githubOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		http.Error(w, "token request failed", http.StatusBadGateway)
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	_ = json.Unmarshal(body, &token)
	if token.AccessToken == "" {
		http.Error(w, "token exchange rejected", http.StatusBadGateway)
		return
	}
	login := ""
	if u, err := githubAPIGet(token.AccessToken, "/user"); err == nil {
		login, _ = u["login"].(string)
	}
	conn := githubOAuthConnection{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		Scope:       token.Scope,
		Login:       login,
		ConnectedAt: time.Now().UTC(),
	}
	if err := a.storeGitHubConnection(st.UserID, conn); err != nil {
		http.Error(w, "could not store connection", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/deploy-apps?github=connected", http.StatusFound)
}

func (a *API) githubOAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ApplicationsWrite) {
		return
	}
	a.deleteGitHubConnection(actor(r).UserID)
	a.audit(r, "github.oauth.disconnect", "integration", "github", true, nil, nil)
	writeJSON(w, 200, map[string]any{"ok": true, "connected": false})
}

func (a *API) githubRepos(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.ApplicationsRead) {
		return
	}
	conn, err := a.loadGitHubConnection(actor(r).UserID)
	if err != nil || conn.AccessToken == "" {
		a.fail(w, r, 400, "NOT_CONNECTED", "Connect GitHub before listing repositories", false)
		return
	}
	page := strings.TrimSpace(r.URL.Query().Get("page"))
	if page == "" {
		page = "1"
	}
	path := "/user/repos?per_page=50&sort=updated&affiliation=owner,collaborator,organization_member&page=" + url.QueryEscape(page)
	req, err := http.NewRequest(http.MethodGet, githubAPIBase+path, nil)
	if err != nil {
		a.fail(w, r, 500, "REQUEST_ERROR", "Could not build GitHub request", true)
		return
	}
	req.Header.Set("Authorization", "Bearer "+conn.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.fail(w, r, 502, "GITHUB_UNAVAILABLE", "GitHub API request failed", true)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		a.fail(w, r, 502, "GITHUB_ERROR", fmt.Sprintf("GitHub returned %d", resp.StatusCode), false)
		return
	}
	var repos []map[string]any
	if err := json.Unmarshal(body, &repos); err != nil {
		a.fail(w, r, 502, "GITHUB_ERROR", "Could not parse GitHub response", false)
		return
	}
	items := make([]map[string]any, 0, len(repos))
	for _, repo := range repos {
		items = append(items, map[string]any{
			"id":            repo["id"],
			"full_name":     repo["full_name"],
			"name":          repo["name"],
			"private":       repo["private"],
			"default_branch": repo["default_branch"],
			"clone_url":     repo["clone_url"],
			"ssh_url":       repo["ssh_url"],
			"html_url":      repo["html_url"],
		})
	}
	writeJSON(w, 200, map[string]any{"items": items, "login": conn.Login})
}

func (a *API) publicOrigin(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}

func githubAPIGet(token, path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, githubAPIBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
