package httpserver

import (
	"net/http"
	"testing"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/store"
)

func TestConvertAddonRejectsMissingAddonAndQueuesProvision(t *testing.T) {
	srv, st, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "srcadd", "srcadd.example.test", pkg)

	if status, body := doStatus(t, http.MethodPost, srv.URL+"/api/v1/accounts/convert-addon", token, map[string]any{
		"account_id": aid, "addon_domain": "missing.srcadd.example.test",
		"username": "newadd", "package_id": pkg, "owner_password": "TenantPass!2026",
	}); status != http.StatusBadRequest {
		t.Fatalf("missing addon must be refused: %d %v", status, body)
	}

	st.PutDomain(&store.Domain{
		ID: id.New(), AccountID: aid, FQDN: "shop.srcadd.example.test",
		ASCII: "shop.srcadd.example.test", Type: "addon", Status: "active",
	})
	queued := doJSON(t, http.MethodPost, srv.URL+"/api/v1/accounts/convert-addon", token, map[string]any{
		"account_id": aid, "addon_domain": "shop.srcadd.example.test",
		"username": "shopadd", "package_id": pkg, "owner_password": "TenantPass!2026",
	})
	if queued["operation_id"] == nil || queued["resource_id"] == nil {
		t.Fatalf("convert must queue a host job: %v", queued)
	}
	job := st.GetJob(queued["operation_id"].(string))
	if job == nil || job.Type != "account.provision" {
		t.Fatalf("provision job: %+v", job)
	}
	if job.ResourceID == "" {
		t.Fatal("new account resource_id must be set")
	}
	if st.DomainTaken("shop.srcadd.example.test") == false {
		t.Fatal("converted domain must remain on the new account")
	}
	created := st.GetAccount(queued["resource_id"].(string))
	if created == nil || created.Username != "shopadd" || created.PrimaryDomain != "shop.srcadd.example.test" {
		t.Fatalf("new account: %+v", created)
	}
	for _, domain := range st.ListDomains(aid) {
		if domain.ASCII == "shop.srcadd.example.test" {
			t.Fatal("source account must no longer hold the converted addon")
		}
	}
}
