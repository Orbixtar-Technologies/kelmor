package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreJournalPinsIdentitiesAndResumes(t *testing.T) {
	dir := t.TempDir()
	pin := RestorePin{
		ActorID:       "user-1",
		AccountID:     "acct-1",
		BackupID:      "bak-1",
		ObjectKey:     "acme42/one.hpm",
		ManifestHash:  "abc123",
		FormatVersion: FormatHPM3,
		KeyIdentity:   "kid-1",
		KeyVersion:    1,
		Fence:         9,
	}
	j, err := CreateJournal(dir, pin)
	if err != nil {
		t.Fatal(err)
	}
	if j.State != StateRestoreRequested {
		t.Fatalf("state %s", j.State)
	}
	for _, step := range []string{StatePreflighted, StateMaintenance, StateStaged, "commit:" + ComponentHome, StateRestoreVerifying} {
		if err := j.Checkpoint(step); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State != StateRestoreVerifying {
		t.Fatalf("resumed %s", reopened.State)
	}
	if err := reopened.PinsMatch(pin); err != nil {
		t.Fatal(err)
	}
	wrong := pin
	wrong.AccountID = "acct-other"
	if err := reopened.PinsMatch(wrong); err == nil {
		t.Fatal("cross-account pin accepted")
	}
	if err := reopened.Complete(); err != nil {
		t.Fatal(err)
	}
	if reopened.State != StateComplete {
		t.Fatalf("state %s", reopened.State)
	}
}

func TestRestoreJournalRollbackFailureRetainsMaintenance(t *testing.T) {
	dir := t.TempDir()
	j, err := CreateJournal(dir, RestorePin{
		ActorID: "user-1", AccountID: "acct-1", BackupID: "bak-1",
		ObjectKey: "k", ManifestHash: "m", FormatVersion: FormatHPM3,
		KeyIdentity: "kid", Fence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Checkpoint(StateMaintenance); err != nil {
		t.Fatal(err)
	}
	if err := j.Checkpoint("commit:" + ComponentHome); err != nil {
		t.Fatal(err)
	}
	if err := j.FailRollback(errors.New("rename failed")); err != nil {
		t.Fatal(err)
	}
	if j.State != StateManualIntervention || !j.ManualIntervention || !j.Maintenance {
		t.Fatalf("%+v", j)
	}
	if _, err := os.Stat(filepath.Join(dir, journalFileName)); err != nil {
		t.Fatal("journal artifact removed")
	}
	reopened, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State != StateManualIntervention || !reopened.Maintenance {
		t.Fatalf("%+v", reopened)
	}
}

func TestRestoreJournalRejectsStaleAndSubstitutedSource(t *testing.T) {
	dir := t.TempDir()
	pin := RestorePin{
		ActorID: "user-1", AccountID: "acct-1", BackupID: "bak-1",
		ObjectKey: "acct/one.hpm", ManifestHash: "hash-1", FormatVersion: FormatHPM3,
		KeyIdentity: "kid-1", Fence: 3,
	}
	j, err := CreateJournal(dir, pin)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Checkpoint(StateMaintenance); err != nil {
		t.Fatal(err)
	}
	stale := pin
	stale.Fence = 2
	if err := j.PinsMatch(stale); err == nil {
		t.Fatal("stale fence accepted")
	}
	swapped := pin
	swapped.ObjectKey = "acct/other.hpm"
	if err := j.PinsMatch(swapped); err == nil {
		t.Fatal("object substitution accepted")
	}
	revoked := pin
	revoked.ActorID = "user-revoked"
	if err := j.PinsMatch(revoked); err == nil {
		t.Fatal("actor substitution accepted")
	}
}
