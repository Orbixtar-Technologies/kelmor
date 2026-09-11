package operations

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	for _, d := range []string{"public_html", "public_ftp", "apps", "backups", "tmp", "logs", "mail", ".ssh"} {
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

func (h *Host) hardenWebDocroot(username, docroot string) {
	if !h.live() || username == "" || docroot == "" {
		return
	}
	ids, err := lookupUIDGID(username)
	if err != nil {
		return
	}
	real, err := h.resolve(docroot)
	if err != nil {
		return
	}
	gid := webServerGID()
	if gid < 0 {
		gid = ids.gid
	}
	_ = filepath.Walk(real, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			_ = os.Chmod(p, 0o750)
		} else {
			_ = os.Chmod(p, 0o640)
		}
		_ = os.Chown(p, ids.uid, gid)
		return nil
	})
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
		_ = h.placeHomeOnQuotaVolume(username, home)
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
	if err := h.placeHomeOnQuotaVolume(username, home); err != nil {
		return Result{}, err
	}
	if err := hardenSFTPHome(home, uid, gid); err != nil {
		return Result{}, err
	}
	return Result{OK: true, Message: "unix identity created", ObservedState: "exists"}, nil
}

func (h *Host) placeHomeOnQuotaVolume(username, home string) error {
	if err := validate.Username(username); err != nil {
		return err
	}
	if home == "" {
		home = "/home/" + username
	}
	volume := "/var/lib/panel/homes"
	if !mountHasTarget(volume) {
		return nil
	}
	src := filepath.Join(volume, username)
	if err := os.MkdirAll(src, 0o751); err != nil {
		return err
	}
	if mountHasTarget(home) {
		return appendBindFstab(src, home)
	}
	if st, err := os.Stat(home); err == nil && st.IsDir() {
		if empty, err := dirIsEmpty(src); err == nil && empty {
			if err := copyDirContents(home, src); err != nil {
				return err
			}
		}
	} else if err := os.MkdirAll(home, 0o751); err != nil {
		return err
	}
	if err := syscall.Mount(src, home, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind %s -> %s: %w", src, home, err)
	}
	return appendBindFstab(src, home)
}

func (h *Host) releaseQuotaHome(username, home string) {
	if home == "" {
		home = "/home/" + username
	}
	if mountHasTarget(home) {
		_ = syscall.Unmount(home, syscall.MNT_DETACH)
	}
	_ = os.RemoveAll(filepath.Join("/var/lib/panel/homes", username))
	removeFstabLine(" /home/" + username + " ")
}

func mountHasTarget(target string) bool {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	want := " " + target + " "
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, want) || strings.HasSuffix(line, " "+target) {
			return true
		}
	}
	return false
}

func dirIsEmpty(path string) (bool, error) {
	ents, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(ents) == 0, nil
}

func copyDirContents(from, to string) error {
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, info.Mode().Perm()); err != nil {
			return err
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(dst, int(stat.Uid), int(stat.Gid))
		}
		return nil
	})
}

func appendBindFstab(src, dst string) error {
	line := src + " " + dst + " none bind 0 0\n"
	b, err := os.ReadFile("/etc/fstab")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(b), strings.TrimSpace(line)) {
		return nil
	}
	f, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(line)
	_ = f.Close()
	return err
}

func removeFstabLine(needle string) {
	b, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return
	}
	var keep []string
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.Contains(line, needle) {
			continue
		}
		keep = append(keep, line)
	}
	_ = os.WriteFile("/etc/fstab", []byte(strings.Join(keep, "\n")+"\n"), 0o644)
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
	h.releaseQuotaHome(username, "/home/"+username)
	_, _ = runFixed("/usr/sbin/userdel", "-f", "-r", username)
	for i := 0; i < 20; i++ {
		if _, err := user.Lookup(username); err != nil {
			return Result{OK: true, ObservedState: "absent"}, nil
		}
		time.Sleep(200 * time.Millisecond)
		_, _ = runFixed("/usr/sbin/userdel", "-f", username)
	}
	return Result{}, fmt.Errorf("unix user %s still exists after userdel", username)
}
