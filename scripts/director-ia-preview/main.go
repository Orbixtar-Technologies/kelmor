// Memory-store Kelmor Control Plane API for Director chrome review.
// Real handlers, no PostgreSQL. Not a production runtime.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/httpserver"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func main() {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		log.Fatal(err)
	}
	pkgs := st.ListPackages()
	pkgID := ""
	if len(pkgs) > 0 {
		pkgID = pkgs[0].ID
	}
	owner := st.UserByUsername("admin")
	acc := &store.Account{
		ID: "acc-demo", OwnerUserID: owner.ID, Username: "demo",
		PrimaryDomain: "demo.test", LinuxUID: 20000, LinuxGID: 20000,
		PackageID: pkgID, Status: "active", HomePath: "/home/demo",
		ShellClass: "sftp-only", DesiredRevision: 1,
	}
	st.PutAccount(acc)
	st.AddMember(acc.ID, owner.ID)
	st.PutDomain(&store.Domain{
		ID: "dom-demo", AccountID: acc.ID, FQDN: "demo.test", ASCII: "demo.test",
		Type: "primary", DocumentRoot: "/home/demo/public_html", DNSManaged: true, Status: "active",
	})
	_, _ = st.EnqueueJob(&store.Job{
		Type: "account.reconcile", ResourceType: "account", ResourceID: acc.ID,
		Payload: map[string]any{"account_id": acc.ID}, State: "failed",
		LastError: "sandbox agent refused live nginx apply", Progress: 40,
		CreatedAt: time.Now().UTC(),
	})
	root := os.Getenv("PANEL_STATE_DIR")
	if root == "" {
		root = "/tmp/director-ia-preview-host"
	}
	_ = os.MkdirAll(root, 0o755)
	agent := &operations.Host{Root: root}
	api := httpserver.New(st, logging.New("director-ia-preview"), agent)
	addr := os.Getenv("PANEL_API_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	log.Printf("Director IA preview API on %s (memory store, real handlers)", addr)
	log.Fatal(http.ListenAndServe(addr, api.Handler()))
}
