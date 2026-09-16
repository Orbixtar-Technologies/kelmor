package phases

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type countPhase struct {
	name      string
	applies   int
	verifyErr error
}

func (p *countPhase) Name() string { return p.name }

func (p *countPhase) Check(Config) error { return nil }

func (p *countPhase) Apply(Config) error {
	p.applies++
	return nil
}

func (p *countPhase) Verify(Config) error { return p.verifyErr }

func TestRunFailsWhenStateCannotPersistBeforePhase(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := os.Mkdir(statePath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Run(Config{Hostname: "panel.example.test", Dev: true}, statePath, filepath.Join(dir, "install.log.jsonl"), []Phase{
		named{"one", checkNoop, applyNoop, verifyNoop},
	})
	if err == nil {
		t.Fatal("installation reported success after state persist failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "state") {
		t.Fatalf("expected state persist error, got %v", err)
	}
}

func TestRunFailsWhenStateCannotPersistAfterPhase(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	logPath := filepath.Join(dir, "install.log.jsonl")
	sabotage := named{"sabotage", checkNoop, func(Config) error {
		if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return os.Mkdir(statePath, 0o755)
	}, verifyNoop}
	err := Run(Config{Hostname: "panel.example.test", Dev: true}, statePath, logPath, []Phase{
		named{"ok", checkNoop, applyNoop, verifyNoop},
		sabotage,
	})
	if err == nil {
		t.Fatal("installation reported success after post-phase state persist failure")
	}
}

func TestRunFailsWhenLogCannotPersist(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "install.log.jsonl")
	if err := os.Mkdir(logPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Run(Config{Hostname: "panel.example.test", Dev: true}, filepath.Join(dir, "state.json"), logPath, []Phase{
		named{"one", checkNoop, applyNoop, verifyNoop},
	})
	if err == nil {
		t.Fatal("installation reported success after log persist failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "log") {
		t.Fatalf("expected log persist error, got %v", err)
	}
}

func TestRunResumesOnlyWhenFingerprintAndVerifyHold(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	logPath := filepath.Join(dir, "install.log.jsonl")
	phase := &countPhase{name: "work"}
	cfg := Config{Hostname: "panel.example.test", Channel: "stable", Dev: true}
	if err := Run(cfg, statePath, logPath, []Phase{phase}); err != nil {
		t.Fatal(err)
	}
	if phase.applies != 1 {
		t.Fatalf("first apply count = %d", phase.applies)
	}
	if err := Run(cfg, statePath, logPath, []Phase{phase}); err != nil {
		t.Fatal(err)
	}
	if phase.applies != 1 {
		t.Fatalf("matching fingerprint reapplied: %d", phase.applies)
	}

	cfg.Hostname = "other.example.test"
	if err := Run(cfg, statePath, logPath, []Phase{phase}); err != nil {
		t.Fatal(err)
	}
	if phase.applies != 2 {
		t.Fatalf("input change must reapply, got %d", phase.applies)
	}

	phase.verifyErr = errors.New("runtime drifted")
	if err := Run(cfg, statePath, logPath, []Phase{phase}); err == nil {
		t.Fatal("expected verify failure after claimed complete")
	}
	if phase.applies != 3 {
		t.Fatalf("failed verify must reapply, got %d", phase.applies)
	}
}
