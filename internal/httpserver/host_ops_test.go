package httpserver

import (
	"net/http/httptest"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestHostAppsConsoleRuntimesAndPasswords(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	apps := get(t, srv.URL+"/api/v1/server/apps", admin)["items"].([]any)
	if len(apps) < 3 {
		t.Fatalf("apps %v", apps)
	}

	recipes := get(t, srv.URL+"/api/v1/server/console/recipes", admin)["items"].([]any)
	if len(recipes) == 0 {
		t.Fatal("expected host recipes")
	}
	ran := post(t, srv.URL+"/api/v1/server/console", admin, map[string]string{"id": "nginx-test"})
	if ran["ok"] != true && ran["observed_state"] != "staged" {
		t.Fatalf("recipe %v", ran)
	}

	runtimes := get(t, srv.URL+"/api/v1/server/runtimes", admin)["items"].([]any)
	if len(runtimes) != 3 {
		t.Fatalf("runtimes %v", runtimes)
	}
	ensured := post(t, srv.URL+"/api/v1/server/runtimes", admin, map[string]string{"version": "8.4"})
	if ensured["observed_state"] != "staged" && ensured["ok"] != true {
		t.Fatalf("runtime %v", ensured)
	}

	code, body := postStatus(t, srv.URL+"/api/v1/server/root-password", admin, map[string]string{"password": "short"})
	if code != 400 {
		t.Fatalf("short root password %d %v", code, body)
	}
	ok := post(t, srv.URL+"/api/v1/server/root-password", admin, map[string]string{"password": "RootPass!2026"})
	if ok["observed_state"] != "staged" && ok["ok"] != true {
		t.Fatalf("root password %v", ok)
	}
	db := post(t, srv.URL+"/api/v1/server/database-root-password", admin, map[string]string{"password": "DbRoot!2026"})
	if db["observed_state"] != "staged" && db["ok"] != true {
		t.Fatalf("db root %v", db)
	}
}

func TestMailingListUsesAliasExpansion(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)
	pkgs := get(t, srv.URL+"/api/v1/packages", admin)["items"].([]any)
	acc := post(t, srv.URL+"/api/v1/accounts", admin, map[string]string{
		"username": "listco", "primary_domain": "list.test", "package_id": pkgs[0].(map[string]any)["id"].(string),
		"owner_email": "owner@list.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	st.PutMailDomain(&store.MailDomain{ID: "md-list", AccountID: aid, DomainID: firstDomainID(st, aid), Status: "active"})

	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin, map[string]any{
		"domain_id":  "md-list",
		"local_part": "staff",
		"members":    []string{"owner@list.test", "ops@list.test"},
	})
	if created["operation_id"] == nil {
		t.Fatalf("list create %v", created)
	}
	items := get(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("lists %v", items)
	}
	list := items[0].(map[string]any)
	if list["local_part"] != "staff" {
		t.Fatalf("list %v", list)
	}
}

func firstDomainID(st store.Store, accountID string) string {
	for _, domain := range st.ListDomains(accountID) {
		return domain.ID
	}
	return ""
}
