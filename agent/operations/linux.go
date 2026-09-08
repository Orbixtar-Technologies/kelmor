package operations

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func hardenSFTPHome(home string, uid, gid int) error {
	if err := os.Chown(home, 0, 0); err != nil {
		return err
	}
	if err := os.Chmod(home, 0o755); err != nil {
		return err
	}
	for _, d := range []string{"public_html", "apps", "backups", "tmp", "logs", "mail", ".ssh"} {
		p := filepath.Join(home, d)
		mode := os.FileMode(0o750)
		if d == "public_html" {
			mode = 0o755
		}
		_ = os.MkdirAll(p, mode)
		_ = os.Chmod(p, mode)
		_ = os.Chown(p, uid, gid)
	}
	return nil
}

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
		_, _ = runFixed("/usr/sbin/usermod", "-aG", "panel-sftp", username)
		_ = hardenSFTPHome(home, uid, gid)
		return Result{OK: true, Message: "unix identity exists", ObservedState: "exists"}, nil
	}
	if out, err := runFixed("/usr/sbin/groupadd", "-g", "19999", "panel-sftp"); err != nil {
		if _, lookupErr := user.LookupGroup("panel-sftp"); lookupErr != nil {
			return Result{}, fmt.Errorf("groupadd panel-sftp: %s", strings.TrimSpace(string(out)))
		}
	}
	if out, err := runFixed("/usr/sbin/groupadd", "-g", strconv.Itoa(gid), username); err != nil {
		if _, lookupErr := user.LookupGroup(username); lookupErr != nil {
			if out2, err2 := runFixed("/usr/sbin/groupadd", username); err2 != nil {
				if _, l2 := user.LookupGroup(username); l2 != nil {
					return Result{}, fmt.Errorf("groupadd: %s / %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
				}
			}
		}
	}
	args := []string{"-u", strconv.Itoa(uid), "-g", username, "-d", home, "-s", shell, "-m", "-G", "panel-sftp", username}
	if out, err := runFixed("/usr/sbin/useradd", args...); err != nil && !strings.Contains(string(out), "already exists") {
		return Result{}, fmt.Errorf("useradd: %s", strings.TrimSpace(string(out)))
	}
	if _, err := user.Lookup(username); err == nil {
		_, _ = runFixed("/usr/sbin/usermod", "-aG", "panel-sftp", username)
	}
	if _, err := h.CreateLinuxUser(username, uid, gid, home, shell); err != nil {
		return Result{}, err
	}
	if err := hardenSFTPHome(home, uid, gid); err != nil {
		return Result{}, err
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
