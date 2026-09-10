package dns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanonicalRecordName(t *testing.T) {
	zone := "orbixtar.dpdns.org"
	cases := map[string]string{
		"@":                         "orbixtar.dpdns.org.",
		"mail":                      "mail.orbixtar.dpdns.org.",
		"mail.orbixtar.dpdns.org":   "mail.orbixtar.dpdns.org.",
		"mail.orbixtar.dpdns.org.":  "mail.orbixtar.dpdns.org.",
	}
	for input, want := range cases {
		if got := canonicalRecordName(zone, input); got != want {
			t.Fatalf("%q => %q, want %q", input, got, want)
		}
	}
}

func TestCreateZoneTreatsExistingZoneAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/servers/localhost/zones" {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := &PowerDNS{BaseURL: server.URL, APIKey: "test-key"}
	if err := provider.CreateZone(context.Background(), "orbixtar.dpdns.org"); err != nil {
		t.Fatalf("existing zone should be idempotent: %v", err)
	}
}
