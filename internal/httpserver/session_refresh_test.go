package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestRefreshRotatesAuthenticatedSession(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})
	oldToken, _ := login["token"].(string)
	if oldToken == "" {
		t.Fatalf("login: %#v", login)
	}

	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/refresh", oldToken, map[string]any{}, nil)
	if code != http.StatusOK {
		t.Fatalf("refresh status %d %#v", code, body)
	}
	newToken, _ := body["token"].(string)
	if newToken == "" || newToken == oldToken {
		t.Fatalf("refresh did not rotate token: %#v", body)
	}
	if get(t, srv.URL+"/api/v1/me", newToken)["user"] == nil {
		t.Fatalf("rotated session cannot call /me: %#v", body)
	}
	code, replay := requestJSONStatus(t, http.MethodGet, srv.URL+"/api/v1/me", oldToken, nil, nil)
	assertAPIErrorCode(t, code, replay, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func TestRefreshRejectsLoginCredentialsAndMissingSession(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/refresh", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	}, nil)
	if code == http.StatusOK {
		t.Fatal("refresh accepted login credentials")
	}
	if code != http.StatusBadRequest && code != http.StatusUnauthorized {
		t.Fatalf("unexpected refresh-with-password status %d %#v", code, body)
	}

	code, body = requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/refresh", "", nil, nil)
	assertAPIErrorCode(t, code, body, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func TestRefreshRejectsExpiredAndAPIToken(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	user := st.UserByUsername("admin")
	plain, hash, err := auth.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	st.PutSession(&store.Session{
		ID: "expired-session", UserID: user.ID, TokenHash: hash,
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	code, body := requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/refresh", plain, map[string]any{}, nil)
	assertAPIErrorCode(t, code, body, http.StatusUnauthorized, "UNAUTHENTICATED")

	tokenPlain, tokenHash, err := auth.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	live := "hp_live_" + tokenPlain
	st.PutToken(&store.APIToken{
		ID: "tok1", UserID: user.ID, TokenHash: auth.HashToken(live),
		Capabilities: []string{"accounts.read"},
	})
	_ = tokenHash
	code, body = requestJSONStatus(t, http.MethodPost, srv.URL+"/api/v1/auth/refresh", live, map[string]any{}, nil)
	if code == http.StatusOK {
		t.Fatal("API token refreshed a session")
	}
}

func TestPublicHealthAndVersionRemainUnauthenticated(t *testing.T) {
	api := New(store.NewMemory(), logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	for _, path := range []string{"/healthz", "/readyz", "/api/v1/version", "/openapi.yaml"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d", path, res.StatusCode)
		}
	}
	res, err := http.Get(srv.URL + "/api/v1/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("healthz must live at the root server URL, not /api/v1")
	}
}
