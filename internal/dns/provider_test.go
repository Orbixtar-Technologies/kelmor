package dns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
