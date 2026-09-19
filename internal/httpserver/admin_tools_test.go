package httpserver

import "testing"

func TestAdminToolsURLsAreHTTPS(t *testing.T) {
	srv, _, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "shop", "shop.test", pkg)
	tools := get(t, srv.URL+"/api/v1/accounts/"+aid+"/admin-tools", token)
	if tools["webmail_url"] != "https://webmail.shop.test/" {
		t.Fatalf("webmail %v", tools)
	}
	if tools["phpmyadmin_url"] != "https://phpmyadmin.shop.test/" {
		t.Fatalf("phpmyadmin %v", tools)
	}
}

func TestImpersonateReturnsControlURLFromPortalHostname(t *testing.T) {
	t.Setenv("PANEL_HOSTNAME", "kelmor.host")
	srv, _, token := directorFixture(t)
	pkg := firstPackageID(t, srv, token)
	aid := seedAccount(t, srv, token, "orbixtar", "orbixtar.test", pkg)
	result := post(t, srv.URL+"/api/v1/accounts/"+aid+"/impersonate", token, map[string]string{
		"reason": "Operator requested Kelmor Control access",
	})
	if result["token"] == nil {
		t.Fatalf("token %v", result)
	}
	if result["control_url"] != "https://kelmor.host:2083/" {
		t.Fatalf("control_url %v", result["control_url"])
	}
}
