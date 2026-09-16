package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestServerMonitorReturnsCachedSamples(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	collected := time.Now().UTC().Add(-2 * time.Minute)
	st.PutAccount(&store.Account{ID: "acc-mon", Username: "mon1", Status: "active", HomePath: "/home/mon1"})
	st.PutUsage(&store.Usage{AccountID: "acc-mon", CollectedAt: collected, DiskBytes: 42, DiskLimited: true})
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	body := get(t, srv.URL+"/api/v1/server/monitor", login["token"].(string))
	accounts, _ := body["accounts"].([]any)
	if len(accounts) == 0 {
		t.Fatalf("monitor: %#v", body)
	}
	found := false
	for _, raw := range accounts {
		item := raw.(map[string]any)
		if item["account_id"] == "acc-mon" {
			found = true
			if item["disk_bytes"].(float64) != 42 || item["disk_limited"] != true {
				t.Fatalf("sample: %#v", item)
			}
			if item["stale_seconds"].(float64) < 60 {
				t.Fatalf("staleness: %#v", item)
			}
		}
	}
	if !found {
		t.Fatalf("cached sample missing: %#v", body)
	}
}

func TestAccountsPageUsesCursor(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"curaaa", "curbbb", "curccc"} {
		st.PutAccount(&store.Account{ID: name, Username: name, Status: "active", PrimaryDomain: name + ".test"})
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	token := login["token"].(string)
	page := get(t, srv.URL+"/api/v1/accounts?limit=1", token)
	if page["has_more"] != true {
		t.Fatalf("expected has_more: %#v", page)
	}
	cursor, _ := page["next_cursor"].(string)
	next := get(t, srv.URL+"/api/v1/accounts?limit=1&cursor="+cursor, token)
	first := page["items"].([]any)[0].(map[string]any)["username"]
	second := next["items"].([]any)[0].(map[string]any)["username"]
	if first == second {
		t.Fatalf("cursor repeated %v", first)
	}
}

func TestAuditSearchUsesCursorNotExactTotal(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		st.AppendAudit(store.AuditEvent{
			ID: store.NewID(), Action: "account.inspect", Success: true,
			OccurredAt: now.Add(time.Duration(i) * time.Second),
		})
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	login := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "ChangeMeOnce!2026"})
	page := get(t, srv.URL+"/api/v1/audit-events?limit=1", login["token"].(string))
	if page["has_more"] != true {
		t.Fatalf("%#v", page)
	}
	if _, err := json.Marshal(page["next_cursor"]); err != nil {
		t.Fatal(err)
	}
	if _, ok := page["items"].([]any); !ok {
		t.Fatalf("items: %#v", page)
	}
	_ = http.StatusOK
}
