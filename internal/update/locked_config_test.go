package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecuteWithLockedConfigLoadsAndWritesStatusUnderCanonicalLock(t *testing.T) {
	installRoot := t.TempDir()
	statusPath := filepath.Join(t.TempDir(), "update-status.json")
	initial, err := acquireInstallLock(installRoot)
	if err != nil {
		t.Fatal(err)
	}

	loaderCalled := make(chan struct{})
	allowLoaderReturn := make(chan struct{})
	executeResult := make(chan error, 1)
	go func() {
		_, executeErr := ExecuteWithLockedConfig(
			context.Background(),
			installRoot,
			"run",
			func() (Config, error) {
				close(loaderCalled)
				<-allowLoaderReturn
				return Config{
					InstallRoot: installRoot,
					StatusPath:  statusPath,
					Channel:     "stable",
					Automatic:   false,
				}, nil
			},
			lockedConfigNoOpRunner{},
		)
		executeResult <- executeErr
	}()

	select {
	case <-loaderCalled:
		_ = initial.Close()
		t.Fatal("config loaded before the canonical lock was acquired")
	case <-time.After(50 * time.Millisecond):
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	<-loaderCalled

	waiterAcquired := make(chan struct{})
	waiterResult := make(chan error, 1)
	go func() {
		waiterResult <- WithInstallOperationLock(installRoot, func() error {
			close(waiterAcquired)
			return nil
		})
	}()
	select {
	case <-waiterAcquired:
		close(allowLoaderReturn)
		t.Fatal("canonical lock was released while config execution was active")
	case <-time.After(50 * time.Millisecond):
	}
	close(allowLoaderReturn)
	if err := <-executeResult; err != nil {
		t.Fatal(err)
	}
	if err := <-waiterResult; err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("disabled status was not written under the operation lock")
	}
}

type lockedConfigNoOpRunner struct{}

func (lockedConfigNoOpRunner) Run(context.Context, string, ...string) error {
	return nil
}
