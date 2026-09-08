package store

import (
	"context"
	"os"
	"testing"
)

func TestPostgresDesiredState(t *testing.T) {
	dsn := os.Getenv("PANEL_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	ctx := context.Background()
	pg, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Skip(err)
	}
	pg.SeedDatabaseServers()
	if err := SeedDev(pg, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	if pg.UserByUsername("admin") == nil {
		t.Fatal("admin missing")
	}
	if len(pg.ListPackages()) == 0 {
		t.Fatal("packages missing")
	}
	job, err := pg.EnqueueJob(&Job{Type: "account.provision", Payload: map[string]any{"k": "v"}, State: "queued", IdempotencyKey: "store-test-claim"})
	if err != nil || job == nil {
		t.Fatalf("enqueue %v %v", job, err)
	}
	claimed := pg.ClaimJob("tester")
	if claimed == nil || claimed.State != "running" {
		t.Fatalf("claim %+v", claimed)
	}
	if claimed.ID != job.ID {
		claimed.State = "queued"
		claimed.LockedBy = ""
		claimed.Attempts = claimed.Attempts - 1
		if claimed.Attempts < 0 {
			claimed.Attempts = 0
		}
		pg.UpdateJob(claimed)
		t.Fatalf("claimed unrelated job %s instead of %s", claimed.ID, job.ID)
	}
	claimed.State = "succeeded"
	now := claimed.CreatedAt
	claimed.FinishedAt = &now
	pg.UpdateJob(claimed)
}
