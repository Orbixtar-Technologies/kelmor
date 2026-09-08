package operations

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func (h *Host) createUnixIdentity(username string, uid, gid int, home, shell string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if uid < 20000 || uid > 199999 || gid < 20000 || gid > 199999 {
		return Result{}, fmt.Errorf("UID/GID outside panel allocation")
	}
	switch shell {
	case "/usr/sbin/nologin", "/bin/bash", "/usr/sbin/rssh", "":
	default:
		return Result{}, fmt.Errorf("shell not permitted")
	}
	if shell == "" {
		shell = "/usr/sbin/nologin"
	}
	if !h.live() {
		return h.CreateLinuxUser(username, uid, gid, home, shell)
	}
	if _, err := user.Lookup(username); err == nil {
		return Result{OK: true, Message: "unix identity exists", ObservedState: "exists"}, nil
	}
	if out, err := runFixed("/usr/sbin/groupadd", "-g", strconv.Itoa(gid), username); err != nil && !strings.Contains(string(out), "already exists") {
		return Result{}, fmt.Errorf("groupadd: %s", strings.TrimSpace(string(out)))
	}
	args := []string{"-u", strconv.Itoa(uid), "-g", strconv.Itoa(gid), "-d", home, "-s", shell, "-m", username}
	if out, err := runFixed("/usr/sbin/useradd", args...); err != nil && !strings.Contains(string(out), "already exists") {
		return Result{}, fmt.Errorf("useradd: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, Message: "unix identity created", ObservedState: "exists"}, nil
}

func (h *Host) lockUnixUser(username string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return Result{OK: true, Message: "locked " + username, ObservedState: "locked"}, nil
	}
	if out, err := runFixed("/usr/sbin/usermod", "-L", "-s", "/usr/sbin/nologin", username); err != nil {
		return Result{}, fmt.Errorf("usermod: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, ObservedState: "locked"}, nil
}

func (h *Host) unlockUnixUser(username string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return Result{OK: true, ObservedState: "unlocked"}, nil
	}
	if out, err := runFixed("/usr/sbin/usermod", "-U", username); err != nil {
		return Result{}, fmt.Errorf("usermod: %s", strings.TrimSpace(string(out)))
	}
	return Result{OK: true, ObservedState: "unlocked"}, nil
}

func (h *Host) deleteUnixUser(username string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return h.DeleteLinuxUser(username)
	}
	_, _ = runFixed("/usr/sbin/userdel", "-r", username)
	return Result{OK: true, ObservedState: "absent"}, nil
}
