package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestMailDeliveryReportAndTrack(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	logPath := filepath.Join(root, "var/log/mail.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "Sep 19 04:12:00 host postfix/qmgr[11]: ABC123: from=<shop@shop.test>, size=512, nrcpt=1 (queue active)\n" +
		"Sep 19 04:12:01 host postfix/smtp[12]: ABC123: to=<user@example.com>, relay=mx.example.com[203.0.113.10]:25, dsn=2.0.0, status=sent (250 OK)\n"
	if err := os.WriteFile(logPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: root})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	admin := post(t, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"username": "admin", "password": "ChangeMeOnce!2026",
	})["token"].(string)

	if status, _ := doStatus(t, http.MethodGet, srv.URL+"/api/v1/mail/delivery-reports?year=2026&month=2&day=31", admin, nil); status != 400 {
		t.Fatalf("invalid date status %d", status)
	}

	report := get(t, srv.URL+"/api/v1/mail/delivery-reports?year=2026&month=9&day=19", admin)
	items, _ := report["items"].([]any)
	if report["date"] != "2026-09-19" || len(items) == 0 {
		t.Fatalf("report %v", report)
	}

	track := get(t, srv.URL+"/api/v1/mail/delivery-track?q=user@example.com", admin)
	tracked, _ := track["items"].([]any)
	if track["mode"] != "track" || len(tracked) == 0 {
		t.Fatalf("track %v", track)
	}
}
