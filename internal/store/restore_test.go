package store

import (
	"errors"
	"testing"
)

func TestBeginRestoreIsAccountUnique(t *testing.T) {
	m := seededRestoreStore(t)
	first := &RestoreJournal{
		AccountID: "a1", BackupID: "bak-1", ActorID: "user-1",
		ObjectKey: "acct/one.hpm", FormatVersion: 3, KeyIdentity: "kid",
		State: StateRestoreRequested,
	}
	got, err := m.BeginRestore(first)
	if err != nil || got.ID == "" {
		t.Fatalf("%+v %v", got, err)
	}
	_, err = m.BeginRestore(&RestoreJournal{
		AccountID: "a1", BackupID: "bak-2", ActorID: "user-1",
		ObjectKey: "acct/two.hpm", State: StateRestoreRequested,
	})
	if !errors.Is(err, ErrRestoreInProgress) {
		t.Fatalf("got %v", err)
	}
	same, err := m.BeginRestore(&RestoreJournal{
		AccountID: "a1", BackupID: "bak-1", ActorID: "user-1",
		ObjectKey: "acct/one.hpm", State: StateRestoreRequested,
	})
	if err != nil || same.ID != got.ID {
		t.Fatalf("resume %v %v", same, err)
	}
}

func TestCheckpointAndFailRestore(t *testing.T) {
	m := seededRestoreStore(t)
	j, err := m.BeginRestore(&RestoreJournal{
		AccountID: "a1", BackupID: "bak-1", ActorID: "user-1",
		ObjectKey: "k", State: StateRestoreRequested, Fence: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.CheckpointRestore(j.ID, StateMaintenance, map[string]any{"component": "home"}); err != nil {
		t.Fatal(err)
	}
	got := m.GetRestore(j.ID)
	if got == nil || got.State != StateMaintenance {
		t.Fatalf("%+v", got)
	}
	if err := m.FailRestore(j.ID, StateManualIntervention, true); err != nil {
		t.Fatal(err)
	}
	got = m.GetRestore(j.ID)
	if got.State != StateManualIntervention || !got.ManualIntervention {
		t.Fatalf("%+v", got)
	}
	if active := m.GetActiveRestore("a1"); active == nil || active.State != StateManualIntervention {
		t.Fatalf("active %+v", active)
	}
}

func seededRestoreStore(t *testing.T) *Memory {
	t.Helper()
	m := NewMemory()
	m.PutAccount(&Account{ID: "a1", Username: "acme42", HomePath: "/home/acme42"})
	m.PutBackup(&BackupRun{ID: "bak-1", AccountID: "a1", Kind: "full", State: "succeeded", Destination: "local"})
	m.PutBackup(&BackupRun{ID: "bak-2", AccountID: "a1", Kind: "full", State: "succeeded", Destination: "local"})
	return m
}
