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

func TestServerJobsEnqueueOnUUIDConstrainedStore(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		body    any
		jobType string
		target  string
	}{
		{
			name: "dns.synchronize", path: "/api/v1/dns/synchronize",
			body: map[string]any{}, jobType: "dns.synchronize", target: "all",
		},
		{
			name: "dns.cleanup", path: "/api/v1/dns/cleanup",
			body: map[string]any{}, jobType: "dns.cleanup", target: "orphans",
		},
		{
			name: "php.runtime.ensure", path: "/api/v1/server/runtimes",
			body: map[string]any{"version": "8.4"}, jobType: "php.runtime.ensure", target: "8.4",
		},
		{
			name: "mail.notify", path: "/api/v1/mail/notify",
			body: map[string]any{
				"from": "ops@example.test", "subject": "Notice",
				"body": "Hello operators", "audience": "owners",
			},
			jobType: "mail.notify", target: "owners",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner := store.NewMemory()
			if err := store.SeedDev(inner, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
				t.Fatal(err)
			}
			st := uuidConstrainedJobStore{Store: inner}
			api := New(st, logging.New("test"), &operations.Host{Sock: "/run/panel/agent.sock"})
			srv := httptest.NewServer(api.Handler())
			t.Cleanup(srv.Close)
			token := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
				"username": "admin", "password": "ChangeMeOnce!2026",
			})["token"].(string)

			saved := doJSON(t, http.MethodPost, srv.URL+tc.path, token, tc.body)
			opID, _ := saved["operation_id"].(string)
			if opID == "" {
				t.Fatalf("expected queued job: %v", saved)
			}
			job := inner.GetJob(opID)
			if job == nil || job.Type != tc.jobType || job.State != "queued" {
				t.Fatalf("queued job: %+v", job)
			}
			if job.ResourceID != "" {
				if _, err := id.Parse(job.ResourceID); err != nil {
					t.Fatalf("jobs.resource_id must be empty or a UUID on Postgres: %q", job.ResourceID)
				}
			}
			if job.Payload["target"] != tc.target {
				t.Fatalf("logical target must stay in payload: %+v", job.Payload)
			}
		})
	}
}
