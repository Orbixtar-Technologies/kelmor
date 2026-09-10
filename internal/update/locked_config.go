package update

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ExecuteWithLockedConfig acquires the canonical install-root lock before
// loading configuration and retains it through the operation and status write.
func ExecuteWithLockedConfig(
	ctx context.Context,
	installRoot string,
	command string,
	load func() (Config, error),
	runner Runner,
) (status *Status, err error) {
	switch command {
	case "check", "install", "run":
	default:
		return nil, fmt.Errorf("unsupported remote command %q", command)
	}
	if load == nil {
		return nil, fmt.Errorf("update config loader is required")
	}

	lock, err := acquireOperationLock(installRoot, unix.LOCK_EX)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, lock.Close())
	}()

	config, err := load()
	if err != nil {
		return nil, err
	}
	if filepath.Clean(config.InstallRoot) != filepath.Clean(installRoot) {
		return nil, fmt.Errorf("configured install root does not match locked install root")
	}
	config.RecoveryRunner = runner

	switch command {
	case "check":
		return checkWithOperationLock(ctx, config, lock)
	case "install":
		return installWithOperationLock(ctx, config, runner, lock)
	case "run":
		if config.Automatic {
			return installWithOperationLock(ctx, config, runner, lock)
		}
		return finishStatus(config.StatusPath, Status{
			State:            "disabled",
			InstalledRelease: config.InstalledRelease,
			Automatic:        false,
			Channel:          config.Channel,
		}, nil)
	default:
		panic("validated update command was not handled")
	}
}
