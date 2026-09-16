package phases

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hosting-panel/panel/internal/releaseversion"
)

func PhaseFingerprint(name string, cfg Config) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "%s\n%s\n%s\n%s\n%s\n%s\n%t\n%s\n",
		name, cfg.Hostname, cfg.Channel, cfg.Root, cfg.ACMEMode,
		releaseversion.Current(), cfg.Dev, cfg.AdminEmail)
	return hex.EncodeToString(sum.Sum(nil))
}

func Run(cfg Config, statePath, logPath string, phases []Phase) error {
	st, err := LoadState(statePath)
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	if st.Fingerprints == nil {
		st.Fingerprints = map[string]string{}
	}
	if st.Attempts == nil {
		st.Attempts = map[string]int{}
	}
	st.Release = releaseversion.Current()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}
	defer log.Close()

	appendLog := func(phase, event string, extra map[string]any) error {
		record := map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"phase":     phase,
			"event":     event,
		}
		for key, value := range extra {
			record[key] = value
		}
		body, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := log.Write(append(body, '\n')); err != nil {
			return err
		}
		return log.Sync()
	}

	for _, phase := range phases {
		fingerprint := PhaseFingerprint(phase.Name(), cfg)
		if st.Phases[phase.Name()] == "complete" && st.Fingerprints[phase.Name()] == fingerprint {
			if err := phase.Verify(cfg); err == nil {
				if err := appendLog(phase.Name(), "skip", map[string]any{"reason": "fingerprint and verify hold"}); err != nil {
					return fmt.Errorf("log: %w", err)
				}
				fmt.Fprintf(os.Stdout, "kelmor-install: %s skip\n", phase.Name())
				continue
			}
			if err := appendLog(phase.Name(), "reapply", map[string]any{"reason": "verify failed after claimed complete"}); err != nil {
				return fmt.Errorf("log: %w", err)
			}
		}
		st.Phases[phase.Name()] = "running"
		st.Attempts[phase.Name()]++
		if err := st.Save(statePath); err != nil {
			return fmt.Errorf("state: %w", err)
		}
		if err := appendLog(phase.Name(), "start", nil); err != nil {
			return fmt.Errorf("log: %w", err)
		}
		fmt.Fprintf(os.Stdout, "kelmor-install: %s start\n", phase.Name())
		if err := phase.Check(cfg); err != nil {
			return failPhase(st, statePath, appendLog, phase.Name(), err)
		}
		if err := phase.Apply(cfg); err != nil {
			if rollbacker, ok := phase.(Rollbacker); ok {
				_ = rollbacker.Rollback(cfg)
			}
			return failPhase(st, statePath, appendLog, phase.Name(), err)
		}
		if err := phase.Verify(cfg); err != nil {
			return failPhase(st, statePath, appendLog, phase.Name(), err)
		}
		st.Phases[phase.Name()] = "complete"
		st.Fingerprints[phase.Name()] = fingerprint
		if err := st.Save(statePath); err != nil {
			return fmt.Errorf("state: %w", err)
		}
		if err := appendLog(phase.Name(), "complete", nil); err != nil {
			return fmt.Errorf("log: %w", err)
		}
		fmt.Fprintf(os.Stdout, "kelmor-install: %s complete\n", phase.Name())
	}
	return nil
}

func failPhase(st *State, statePath string, appendLog func(string, string, map[string]any) error, name string, applyErr error) error {
	st.Phases[name] = "failed"
	saveErr := st.Save(statePath)
	logErr := appendLog(name, "failed", map[string]any{"error": applyErr.Error()})
	if saveErr != nil {
		return fmt.Errorf("phase %s failed: %w; state: %v", name, applyErr, saveErr)
	}
	if logErr != nil {
		return fmt.Errorf("phase %s failed: %w; log: %v", name, applyErr, logErr)
	}
	return fmt.Errorf("phase %s failed: %w", name, applyErr)
}
