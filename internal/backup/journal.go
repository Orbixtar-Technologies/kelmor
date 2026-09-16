package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	journalFileName         = "restore.journal"
	StateRestoreRequested   = "restore_requested"
	StatePreflighted        = "preflighted"
	StateMaintenance        = "maintenance"
	StateStaged             = "staged"
	StateCommitting         = "committing"
	StateRestoreVerifying   = "restore_verifying"
	StateComplete           = "complete"
	StateRollingBack        = "rolling_back"
	StateManualIntervention = "manual_intervention"
	StateFailed             = "failed"
)

type RestorePin struct {
	ActorID       string `json:"actor_id"`
	AccountID     string `json:"account_id"`
	BackupID      string `json:"backup_id"`
	ObjectKey     string `json:"object_key"`
	ManifestHash  string `json:"manifest_hash"`
	FormatVersion int    `json:"format_version"`
	KeyIdentity   string `json:"key_identity"`
	KeyVersion    uint32 `json:"key_version"`
	Fence         int64  `json:"fence"`
}

type Journal struct {
	Pin                RestorePin `json:"pin"`
	State              string     `json:"state"`
	Checkpoints        []string   `json:"checkpoints"`
	ManualIntervention bool       `json:"manual_intervention"`
	Maintenance        bool       `json:"maintenance"`
	Error              string     `json:"error,omitempty"`
	UpdatedAt          string     `json:"updated_at"`
	path               string     `json:"-"`
}

func CreateJournal(dir string, pin RestorePin) (*Journal, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	j := &Journal{
		Pin:         pin,
		State:       StateRestoreRequested,
		Checkpoints: []string{StateRestoreRequested},
		path:        filepath.Join(dir, journalFileName),
	}
	if err := j.persist(); err != nil {
		return nil, err
	}
	return j, nil
}

func OpenJournal(dir string) (*Journal, error) {
	path := filepath.Join(dir, journalFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var j Journal
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, err
	}
	j.path = path
	return &j, nil
}

func (j *Journal) Checkpoint(step string) error {
	if j == nil {
		return fmt.Errorf("restore journal is required")
	}
	j.State = step
	if step == StateMaintenance {
		j.Maintenance = true
	}
	j.Checkpoints = append(j.Checkpoints, step)
	return j.persist()
}

func (j *Journal) Complete() error {
	j.State = StateComplete
	j.Maintenance = false
	j.Checkpoints = append(j.Checkpoints, StateComplete)
	return j.persist()
}

func (j *Journal) FailRollback(cause error) error {
	j.State = StateManualIntervention
	j.ManualIntervention = true
	j.Maintenance = true
	if cause != nil {
		j.Error = cause.Error()
	}
	j.Checkpoints = append(j.Checkpoints, StateManualIntervention)
	return j.persist()
}

func (j *Journal) PinsMatch(pin RestorePin) error {
	if j.Pin.ActorID != pin.ActorID {
		return fmt.Errorf("restore actor mismatch")
	}
	if j.Pin.AccountID != pin.AccountID {
		return fmt.Errorf("restore account mismatch")
	}
	if j.Pin.BackupID != pin.BackupID {
		return fmt.Errorf("restore backup mismatch")
	}
	if j.Pin.ObjectKey != pin.ObjectKey {
		return fmt.Errorf("restore object substitution")
	}
	if j.Pin.ManifestHash != pin.ManifestHash {
		return fmt.Errorf("restore manifest mismatch")
	}
	if j.Pin.FormatVersion != pin.FormatVersion {
		return fmt.Errorf("restore format mismatch")
	}
	if j.Pin.KeyIdentity != pin.KeyIdentity {
		return fmt.Errorf("restore key identity mismatch")
	}
	if pin.Fence != 0 && j.Pin.Fence != pin.Fence {
		return fmt.Errorf("stale restore fence")
	}
	return nil
}

func (j *Journal) HasCheckpoint(step string) bool {
	for _, item := range j.Checkpoints {
		if item == step {
			return true
		}
	}
	return false
}

func (j *Journal) ResetForTest(state string) error {
	j.State = state
	j.Checkpoints = []string{StateRestoreRequested, state}
	return j.persist()
}

func (j *Journal) persist() error {
	j.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	tmp := j.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, j.path)
}
