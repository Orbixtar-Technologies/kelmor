package job

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/backup"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func TestCreateBackupFailsClosedOnMissingMailbox(t *testing.T) {
	st, w, acc := backupWorker(t)
	st.PutDomain(&store.Domain{ID: "dom-mail", AccountID: acc.ID, FQDN: "bk.test", ASCII: "bk.test", Type: "primary"})
	st.PutMailDomain(&store.MailDomain{ID: "md-1", AccountID: acc.ID, DomainID: "dom-mail"})
	st.PutMailbox(&store.Mailbox{ID: "mb-1", AccountID: acc.ID, DomainID: "dom-mail", LocalPart: "info"})
	b := &store.BackupRun{ID: "bak-mail", AccountID: acc.ID, Kind: "full", State: "queued", Destination: "local"}
	st.PutBackup(b)
	err := w.createBackup(&store.Job{Payload: map[string]any{"backup_id": b.ID}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "mail") {
		t.Fatalf("got %v", err)
	}
	got := st.GetBackup(b.ID)
	if got.State == "succeeded" {
		t.Fatal("failed inventory published success")
	}
}

func TestRestoreJournalResumesAfterComponentCheckpoint(t *testing.T) {
	st, w, acc := backupWorker(t)
	b := &store.BackupRun{ID: "bak-rs2", AccountID: acc.ID, Kind: "full", State: "queued", Destination: "local"}
	st.PutBackup(b)
	if err := w.createBackup(&store.Job{Payload: map[string]any{"backup_id": b.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := w.restoreBackup(&store.Job{
		ID: "job-restore-1", ActorID: "user-1",
		Payload: map[string]any{"backup_id": b.ID, "account_id": acc.ID},
	}); err != nil {
		t.Fatal(err)
	}
	dir := w.restoreJournalDir(acc.ID)
	j, err := backup.OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.ResetForTest(backup.StateStaged); err != nil {
		t.Fatal(err)
	}
	if err := j.Checkpoint("commit:" + backup.ComponentHome); err != nil {
		t.Fatal(err)
	}
	if err := w.restoreBackup(&store.Job{
		ID: "job-restore-1", ActorID: "user-1",
		Payload: map[string]any{"backup_id": b.ID, "account_id": acc.ID},
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := backup.OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State != backup.StateComplete {
		t.Fatalf("resumed %s", reopened.State)
	}
}

func TestRestoreRejectsCrossAccountSubstitution(t *testing.T) {
	st, w, acc := backupWorker(t)
	other := &store.Account{ID: "acc-other", Username: "other42", HomePath: "/home/other42", LinuxUID: 21000, LinuxGID: 21000}
	st.PutAccount(other)
	b := &store.BackupRun{ID: "bak-x", AccountID: acc.ID, Kind: "full", State: "queued", Destination: "local"}
	st.PutBackup(b)
	if err := w.createBackup(&store.Job{Payload: map[string]any{"backup_id": b.ID}}); err != nil {
		t.Fatal(err)
	}
	err := w.restoreBackup(&store.Job{
		ActorID: "user-1",
		Payload: map[string]any{"backup_id": b.ID, "account_id": other.ID},
	})
	if err == nil {
		t.Fatal("cross-account restore succeeded")
	}
}

func TestCreateBackupPersistsObjectKeyBeforeUpload(t *testing.T) {
	st, w, acc := backupWorker(t)
	b := &store.BackupRun{ID: "bak-key", AccountID: acc.ID, Kind: "full", State: "queued", Destination: "local"}
	st.PutBackup(b)
	if err := w.createBackup(&store.Job{Payload: map[string]any{"backup_id": b.ID}}); err != nil {
		t.Fatal(err)
	}
	got := st.GetBackup(b.ID)
	if got.State != "succeeded" {
		t.Fatalf("state %s", got.State)
	}
	key, _ := got.Manifest["key"].(string)
	if key == "" {
		t.Fatal("object key missing")
	}
	root := filepath.Join(w.Agent.Root, "var/lib/panel/backups")
	if _, err := os.Stat(filepath.Join(root, key)); err != nil {
		t.Fatal(err)
	}
}

func backupWorker(t *testing.T) (*store.Memory, *Worker, *store.Account) {
	t.Helper()
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	pkgs := st.ListPackages()
	acc := &store.Account{
		ID: "acc-bk", Username: "bk42", PrimaryDomain: "bk.test",
		PackageID: pkgs[0].ID, Status: "active", HomePath: "/home/bk42",
		LinuxUID: 22001, LinuxGID: 22001, DesiredRevision: 1,
	}
	st.PutAccount(acc)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "home", "bk42", "public_html"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "home", "bk42", "public_html", "index.html"), []byte("site"), 0o644); err != nil {
		t.Fatal(err)
	}
	box, err := secret.FromBytes(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := New(st, &operations.Host{Root: root}, logging.New("test"), box, "tester")
	return st, w, acc
}
