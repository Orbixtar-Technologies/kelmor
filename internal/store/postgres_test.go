package store

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/rbac"
)

func TestPostgresDesiredState(t *testing.T) {
	dsn, explicitlyConfigured := os.LookupEnv("PANEL_DATABASE_URL")
	if !explicitlyConfigured {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	ctx := context.Background()
	pg, err := OpenPostgres(ctx, dsn)
	if err != nil {
		if explicitlyConfigured {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	t.Cleanup(pg.Close)
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
	job, err := pg.EnqueueJob(&Job{Type: "account.provision", Payload: map[string]any{"k": "v"}, State: "queued", IdempotencyKey: "store-test-claim-" + id.New()})
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

	featureSetID := pg.ListFeatureSets()[0].ID
	unusedPackage := &Package{ID: id.New(), Name: "postgres-delete-" + id.New(), FeatureSetID: featureSetID}
	pg.PutPackage(unusedPackage)
	if pg.GetPackage(unusedPackage.ID) == nil {
		t.Fatal("delete test package missing")
	}
	if !pg.DeletePackageIfUnused(unusedPackage.ID) || pg.GetPackage(unusedPackage.ID) != nil {
		t.Fatal("unused package was not deleted")
	}
	if pg.DeletePackageIfUnused(unusedPackage.ID) {
		t.Fatal("missing package reported as deleted")
	}

	userID := id.New()
	user := &User{
		ID: userID, Username: "pg-" + userID, Email: userID + "@postgres.test",
		PasswordHash: "old-password-hash", DisplayName: "Postgres Rotation",
		Status: "active", MustChangePassword: true,
	}
	pg.PutUser(user)
	rotation, err := pg.RotatePasswordAndEnqueue(userID, "new-password-hash", false, &Job{
		ID: id.New(), Type: "account.reconcile", Payload: map[string]any{"account_id": id.New()},
		State: "queued", IdempotencyKey: "postgres-rotation-" + userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	rotated := pg.UserByID(userID)
	if rotated == nil || rotated.PasswordHash != "new-password-hash" || rotated.MustChangePassword {
		t.Fatalf("password rotation was not committed: %+v", rotated)
	}
	if stored := pg.GetJob(rotation.ID); stored == nil || stored.Payload["account_id"] == nil {
		t.Fatalf("password rotation job missing: %+v", stored)
	}
	rotation.State = "succeeded"
	rotation.FinishedAt = &rotation.CreatedAt
	pg.UpdateJob(rotation)

	rollbackUserID := id.New()
	rollbackUser := &User{
		ID: rollbackUserID, Username: "pg-" + rollbackUserID, Email: rollbackUserID + "@postgres.test",
		PasswordHash: "rollback-old-hash", DisplayName: "Postgres Rollback",
		Status: "active", MustChangePassword: true,
	}
	pg.PutUser(rollbackUser)
	if _, err := pg.RotatePasswordAndEnqueue(rollbackUserID, "must-not-commit", false, &Job{
		ID: id.New(), Type: "account.reconcile", ResourceID: "not-a-uuid",
		Payload: map[string]any{}, State: "queued",
	}); err == nil {
		t.Fatal("invalid job unexpectedly committed")
	}
	rolledBack := pg.UserByID(rollbackUserID)
	if rolledBack == nil || rolledBack.PasswordHash != "rollback-old-hash" || !rolledBack.MustChangePassword {
		t.Fatalf("failed enqueue did not roll back password: %+v", rolledBack)
	}

	account := &Account{
		ID: id.New(), OwnerUserID: rollbackUserID, Username: "atomic-" + id.New(),
		PrimaryDomain: "atomic-" + id.New() + ".test", LinuxUID: pg.AllocUID(), PackageID: pg.ListPackages()[0].ID,
		Status: "active", HomePath: "/home/postgres-atomic", ShellClass: "sftp-only", DesiredRevision: 1,
	}
	account.LinuxGID = account.LinuxUID
	pg.PutAccount(account)
	updated := pg.GetAccount(account.ID)
	if updated == nil {
		t.Fatal("atomic update test account missing")
	}
	updated.Status = "suspended"
	updated.DesiredRevision++
	if _, err := pg.UpdateAccountWithJob(updated, &Job{
		ID: "not-a-uuid", Type: "account.reconcile", ResourceType: "account", ResourceID: updated.ID,
		Payload: map[string]any{"account_id": updated.ID},
	}); err == nil {
		t.Fatal("invalid account update job unexpectedly committed")
	}
	if got := pg.GetAccount(account.ID); !reflect.DeepEqual(got, account) {
		t.Fatalf("failed enqueue did not roll back account update:\n got: %+v\nwant: %+v", got, account)
	}

	legacyOwner := &User{
		ID: id.New(), Username: "legacy-reseller-" + id.New(), Email: id.New() + "@postgres.test",
		PasswordHash: "test", DisplayName: "Legacy Reseller", Status: "active",
	}
	pg.PutUser(legacyOwner)
	legacyReseller := &Reseller{
		ID: id.New(), UserID: legacyOwner.ID, Name: "Legacy Reseller",
		PrivilegeMask: []string{}, Nameservers: []string{}, Status: "active",
	}
	pg.PutReseller(legacyReseller)
	migrationName := "000025_backfill_legacy_reseller_privileges.sql"
	if _, err := pg.pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version=$1`, migrationName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pg.pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING`, migrationName)
	})
	if err := Migrate(ctx, pg.pool); err != nil {
		t.Fatal(err)
	}
	if got := pg.GetReseller(legacyReseller.ID); got == nil ||
		!reflect.DeepEqual(got.PrivilegeMask, rbac.RoleCaps["reseller"]) {
		t.Fatalf("legacy reseller privileges were not backfilled: %+v", got)
	}

	newOwner := &User{
		ID: id.New(), Username: "new-reseller-" + id.New(), Email: id.New() + "@postgres.test",
		PasswordHash: "test", DisplayName: "New Reseller", Status: "active",
	}
	pg.PutUser(newOwner)
	newReseller := &Reseller{
		ID: id.New(), UserID: newOwner.ID, Name: "New Reseller",
		PrivilegeMask: []string{}, Nameservers: []string{}, Status: "active",
	}
	pg.PutReseller(newReseller)
	if got := pg.GetReseller(newReseller.ID); got == nil || !reflect.DeepEqual(got.PrivilegeMask, []string{}) {
		t.Fatalf("new empty reseller privileges did not remain empty: %+v", got)
	}
}
