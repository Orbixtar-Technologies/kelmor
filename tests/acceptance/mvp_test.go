package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/httpserver"
	"github.com/hosting-panel/panel/internal/job"
	"github.com/hosting-panel/panel/internal/migration"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func TestMVPAcceptancePath(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	box, err := secret.FromBytes(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	agent := &operations.Host{Root: root}
	api := httpserver.New(st, logging.New("accept"), agent)
	w := job.New(st, agent, logging.New("accept-w"), box, "accept")
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)

	token := login(t, srv.URL, "admin", "ChangeMeOnce!2026")
	pkg := get(t, srv.URL+"/api/v1/packages", token)["items"].([]any)[0].(map[string]any)
	created := post(t, srv.URL+"/api/v1/accounts", token, map[string]any{
		"username": "acme42", "primary_domain": "acme.test", "package_id": pkg["id"],
		"owner_email": "owner@acme.test", "owner_password": "TenantPass!2026",
	})
	aid := created["resource_id"].(string)
	w.Drain(context.Background())
	acc := get(t, srv.URL+"/api/v1/accounts/"+aid, token)
	if acc["status"] != "active" {
		t.Fatalf("provision status %v", acc["status"])
	}
	if _, err := os.Stat(filepath.Join(root, "home/acme42/public_html/index.html")); err != nil {
		t.Fatal("site files", err)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/mail/virtual")); err != nil {
		t.Fatal("mail maps", err)
	}

	mds := get(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/domains", token)["items"].([]any)
	if len(mds) == 0 {
		t.Fatal("mail domain missing")
	}
	md := mds[0].(map[string]any)
	post(t, srv.URL+"/api/v1/accounts/"+aid+"/mail/mailboxes", token, map[string]any{
		"domain_id": md["id"], "local_part": "info", "password": "MailboxPass!2026",
	})
	w.Drain(context.Background())
	if _, err := os.Stat(filepath.Join(root, "var/vmail/acme.test/info/Maildir")); err != nil {
		t.Fatal("mailbox home", err)
	}

	post(t, srv.URL+"/api/v1/accounts/"+aid+"/files", token, map[string]any{
		"path": "/public_html/note.txt", "content": "hosted-file",
	})
	note, err := os.ReadFile(filepath.Join(root, "home/acme42/public_html/note.txt"))
	if err != nil || string(note) != "hosted-file" {
		t.Fatalf("file write %q %v", note, err)
	}

	bak := post(t, srv.URL+"/api/v1/accounts/"+aid+"/backups", token, map[string]any{"kind": "full", "destination": "local"})
	w.Drain(context.Background())
	bid := bak["backup"].(map[string]any)["id"].(string)
	items := get(t, srv.URL+"/api/v1/accounts/"+aid+"/backups", token)["items"].([]any)
	found := false
	for _, it := range items {
		b := it.(map[string]any)
		if b["id"] == bid && b["state"] == "succeeded" {
			found = true
		}
	}
	if !found {
		t.Fatal("backup did not succeed")
	}

	post(t, srv.URL+"/api/v1/accounts/"+aid+"/suspend", token, map[string]any{})
	w.Drain(context.Background())
	if get(t, srv.URL+"/api/v1/accounts/"+aid, token)["status"] != "suspended" {
		t.Fatal("suspend")
	}
	post(t, srv.URL+"/api/v1/accounts/"+aid+"/unsuspend", token, map[string]any{})
	w.Drain(context.Background())

	raw, _ := json.Marshal(mustExport(t, st, aid))
	imp := store.NewMemory()
	got, err := migration.Import(imp, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "acme42" {
		t.Fatal(got)
	}

	audit := get(t, srv.URL+"/api/v1/audit-events", token)["items"].([]any)
	if len(audit) == 0 {
		t.Fatal("audit empty")
	}
	mon := get(t, srv.URL+"/api/v1/server/monitor", token)
	if mon["failed_jobs"] == nil {
		t.Fatal("monitor")
	}
}

func mustExport(t *testing.T, st store.Store, aid string) *migration.HostingAccountExport {
	t.Helper()
	exp, err := migration.Export(st, aid)
	if err != nil {
		t.Fatal(err)
	}
	return exp
}

func login(t *testing.T, base, user, pass string) string {
	t.Helper()
	out := post(t, base+"/api/v1/auth/login", "", map[string]any{"username": user, "password": pass})
	tok, _ := out["token"].(string)
	if tok == "" {
		t.Fatal(out)
	}
	return tok
}

func post(t *testing.T, url, token string, body any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode >= 400 {
		t.Fatalf("%s %d %v", url, res.StatusCode, out)
	}
	return out
}

func get(t *testing.T, url, token string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode >= 400 {
		t.Fatalf("%s %d %v", url, res.StatusCode, out)
	}
	return out
}
