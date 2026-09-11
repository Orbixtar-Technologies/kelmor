package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/id"
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
	if ensured["operation_id"] == nil || ensured["status"] != "provisioning" {
		t.Fatalf("runtime %v", ensured)
	}
	job := st.GetJob(ensured["operation_id"].(string))
	if job == nil || job.Type != "php.runtime.ensure" {
		t.Fatalf("php job %v", job)
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
	code, body = postStatus(t, srv.URL+"/api/v1/server/database-root-password", admin, map[string]string{"password": "-DbRoot!2026"})
	if code != 400 {
		t.Fatalf("dash-prefix db password %d %v", code, body)
	}
}

func TestDatabaseRootPasswordRequiresServerScope(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	putUpdateTestUser(t, st, "tenant-db", "customer_owner")
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	token := loginUpdateTestUser(t, srv.URL, "tenant-db", "UpdateTestPass!2026")
	code, body := postStatus(t, srv.URL+"/api/v1/server/database-root-password", token, map[string]string{
		"password": "DbRoot!2026",
	})
	if code != http.StatusForbidden {
		t.Fatalf("customer database root password %d %v", code, body)
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

	empty := get(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin)["items"].([]any)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty lists %v", empty)
	}

	created := post(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin, map[string]any{
		"domain_id":  "md-list",
		"local_part": "staff",
		"members":    []string{"owner@list.test", "ops@list.test"},
	})
	if created["operation_id"] == nil {
		t.Fatalf("list create %v", created)
	}
	listBody, _ := created["list"].(map[string]any)
	if listBody["status"] != "provisioning" {
		t.Fatalf("create status %v", created)
	}
	items := get(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("lists %v", items)
	}
	list := items[0].(map[string]any)
	if list["local_part"] != "staff" || list["status"] != "active" {
		t.Fatalf("list %v", list)
	}

	solo := post(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin, map[string]any{
		"domain_id":  "md-list",
		"local_part": "solo",
		"members":    []string{"owner@list.test"},
	})
	if solo["operation_id"] == nil {
		t.Fatalf("one-member list %v", solo)
	}

	st.PutMailbox(&store.Mailbox{ID: "mb-info", AccountID: aid, DomainID: "md-list", LocalPart: "info", Status: "active"})
	code, body := postStatus(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists", admin, map[string]any{
		"domain_id":  "md-list",
		"local_part": "info",
		"members":    []string{"owner@list.test", "ops@list.test"},
	})
	if code != http.StatusConflict {
		t.Fatalf("list vs mailbox %d %v", code, body)
	}

	plain := &store.MailAlias{ID: "al-plain", AccountID: aid, DomainID: "md-list", Address: "sales", Destination: "owner@list.test"}
	st.PutMailAlias(plain)
	code, body = requestJSONStatus(t, http.MethodPatch, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists/"+plain.ID, admin, map[string]any{
		"members": []string{"ops@list.test"},
	}, nil)
	if code != http.StatusNotFound {
		t.Fatalf("patch non-list %d %v", code, body)
	}
	code, body = requestJSONStatus(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists/"+plain.ID, admin, nil, nil)
	if code != http.StatusNotFound {
		t.Fatalf("delete non-list %d %v", code, body)
	}
	listID := list["id"].(string)
	if statusOf(t, http.MethodDelete, srv.URL+"/api/v1/accounts/"+aid+"/mail/lists/"+listID, admin, nil) != http.StatusAccepted {
		t.Fatal("delete mailing list")
	}
}

func TestHostAppEnableGuardsWordPressAndAccount(t *testing.T) {
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
		"username": "apptool", "primary_domain": "app.test", "package_id": pkgs[0].(map[string]any)["id"].(string),
		"owner_email": "owner@app.test", "owner_password": "TenantPass!2026",
	})
	aid := acc["resource_id"].(string)
	site := &store.Website{ID: id.New(), AccountID: aid, DocumentRoot: "/home/apptool/public_html", Runtime: "php", Enabled: true}
	st.PutWebsite(site)

	code, body := postStatus(t, srv.URL+"/api/v1/server/apps/phpmyadmin/enable", admin, map[string]any{})
	if code != http.StatusBadRequest {
		t.Fatalf("phpmyadmin without account %d %v", code, body)
	}
	code, body = postStatus(t, srv.URL+"/api/v1/server/apps/wordpress/enable", admin, map[string]any{
		"account_id": aid, "website_id": site.ID,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("wordpress without creds %d %v", code, body)
	}
}

func firstDomainID(st store.Store, accountID string) string {
	for _, domain := range st.ListDomains(accountID) {
		return domain.ID
	}
	return ""
}
