package store

import (
	"context"
	"os"
	"testing"

	"github.com/hosting-panel/panel/internal/id"
)

func openAtomicPostgresTest(t *testing.T) *PG {
	t.Helper()
	dsn, explicitlyConfigured := os.LookupEnv("PANEL_DATABASE_URL")
	if !explicitlyConfigured {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	pg, err := OpenPostgres(context.Background(), dsn)
	if err != nil {
		if explicitlyConfigured {
			t.Fatal(err)
		}
		t.Skipf("PostgreSQL unavailable: %v", err)
	}
	t.Cleanup(pg.Close)
	return pg
}

func TestPostgresImportAccountWithJobRollsBackMalformedTree(t *testing.T) {
	pg := openAtomicPostgresTest(t)
	pg.SeedDatabaseServers()
	if err := SeedDev(pg, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	owner := pg.UserByUsername("admin")
	accountID := id.New()
	username := postgresTestAccountUsername("import-")
	imported := &AccountImport{
		Account: Account{
			ID: accountID, OwnerUserID: owner.ID, Username: username,
			PrimaryDomain: username + ".test", PackageID: pg.ListPackages()[0].ID,
			Status: "provisioning", HomePath: "/home/" + username, ShellClass: "sftp-only",
			DesiredRevision: 1,
		},
		Websites: []Website{{
			ID: id.New(), AccountID: accountID, DomainID: id.New(), Runtime: "php",
			DocumentRoot: "/home/" + username + "/public_html", DesiredRevision: 1,
		}},
	}
	if _, err := pg.ImportAccountWithJob(imported, &Job{
		ID: id.New(), Type: "account.reconcile", ResourceType: "account",
		ResourceID: accountID, State: "queued",
	}); err == nil {
		t.Fatal("malformed PostgreSQL import unexpectedly committed")
	}
	if account := pg.GetAccount(accountID); account != nil {
		t.Fatalf("failed PostgreSQL import left account behind: %+v", account)
	}
}
