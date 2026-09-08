package operations

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

func ownerName(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return ""
	}
	return u.Username
}

func hostingDirMode(path string) os.FileMode {
	// public_html is 0750 (owner + web group). World-readable 0755
	// lets one tenant read another tenant's files.
	_ = path
	return 0o750
}

func webServerGID() int {
	for _, name := range []string{"www-data", "nginx"} {
		g, err := user.LookupGroup(name)
		if err != nil {
			continue
		}
		id, err := strconv.Atoi(g.Gid)
		if err == nil {
			return id
		}
	}
	return -1
}

func hardenSFTPHome(home string, uid, gid int) error {
	if err := os.Chown(home, 0, 0); err != nil {
		return err
	}
	// 0751: nginx can traverse; other tenants cannot list or write.
	// ACL lets the tenant list their own chroot root (OpenSSH forbids
	// making this directory group-writable).
	if err := os.Chmod(home, 0o751); err != nil {
		return err
	}
	if uname := ownerName(uid); uname != "" {
		_, _ = runFixed("/usr/bin/setfacl", "-m", "u:"+uname+":r-x", home)
	}
	webgid := webServerGID()
	for _, d := range []string{"public_html", "apps", "backups", "tmp", "logs", "mail", ".ssh"} {
		p := filepath.Join(home, d)
		mode := os.FileMode(0o750)
		_ = os.MkdirAll(p, mode)
		_ = os.Chmod(p, mode)
		ownGid := gid
		if d == "public_html" && webgid > 0 {
			ownGid = webgid
		}
		_ = os.Chown(p, uid, ownGid)
		if d == "public_html" {
			hardenPublicFiles(p, uid, ownGid)
		}
	}
	return nil
}

func hardenPublicFiles(dir string, uid, gid int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		_ = os.Chmod(p, 0o640)
		_ = os.Chown(p, uid, gid)
	}
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

func (h *Host) setLinuxPassword(username, password string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if len(password) < 8 || strings.ContainsAny(password, "\n\r:") {
		return Result{}, fmt.Errorf("invalid linux password")
	}
	if !h.live() {
		return Result{OK: true, Message: "password staged", ObservedState: "staged"}, nil
	}
	cmd := exec.Command("/usr/sbin/chpasswd")
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "LC_ALL=C"}
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("chpasswd: %s", strings.TrimSpace(string(out)))
	}
	_, _ = runFixed("/usr/sbin/usermod", "-U", username)
	return Result{OK: true, Message: "linux password set", ObservedState: "set"}, nil
}

func (h *Host) deleteUnixUser(username string) (Result, error) {
	if err := validate.Username(username); err != nil {
		return Result{}, err
	}
	if !h.live() {
		return h.DeleteLinuxUser(username)
	}
	_, _ = runFixed("/usr/sbin/userdel", "-f", "-r", username)
	return Result{OK: true, ObservedState: "absent"}, nil
}
