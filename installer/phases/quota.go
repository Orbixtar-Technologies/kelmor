package phases

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	quotaImage = "/var/lib/panel/quota-homes.img"
	quotaMount = "/var/lib/panel/homes"
	quotaBytes = 32 << 30
)

func applyQuotaHomes(c Config) error {
	if c.Dev {
		return os.MkdirAll(root(c, "var/lib/panel/homes"), 0o750)
	}
	return ensureQuotaHomes()
}

func verifyQuotaHomes(c Config) error {
	if c.Dev {
		if _, err := os.Stat(root(c, "var/lib/panel/homes")); err != nil {
			return err
		}
		return nil
	}
	if !pathMounted(quotaMount) {
		return fmt.Errorf("%s is not a mounted quota filesystem", quotaMount)
	}
	return nil
}

func ensureQuotaHomes() error {
	if err := os.MkdirAll("/var/lib/panel", 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(quotaImage); err != nil {
		f, err := os.OpenFile(quotaImage, os.O_CREATE|os.O_RDWR, 0o640)
		if err != nil {
			return err
		}
		if err := f.Truncate(quotaBytes); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
		mkfs := firstBin("/sbin/mkfs.ext4", "/usr/sbin/mkfs.ext4")
		if mkfs == "" {
			return fmt.Errorf("mkfs.ext4 missing")
		}
		cmd := exec.Command(mkfs, "-F", "-O", "quota", quotaImage)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs.ext4: %s", strings.TrimSpace(string(out)))
		}
	}
	if err := os.MkdirAll(quotaMount, 0o755); err != nil {
		return err
	}
	if !pathMounted(quotaMount) {
		cmd := exec.Command("/bin/mount", "-o", "loop,usrquota,grpquota", quotaImage, quotaMount)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mount quota homes: %s", strings.TrimSpace(string(out)))
		}
	}
	quotaon := firstBin("/sbin/quotaon", "/usr/sbin/quotaon")
	if quotaon != "" {
		_ = exec.Command(quotaon, "-ug", quotaMount).Run()
	}
	if err := ensureQuotaFstab(); err != nil {
		return err
	}
	return bindPanelHomes()
}

func bindPanelHomes() error {
	for _, name := range panelAccountNames() {
		src := quotaMount + "/" + name
		dst := "/home/" + name
		if err := os.MkdirAll(src, 0o751); err != nil {
			return err
		}
		if st, err := os.Stat(dst); err == nil && st.IsDir() && !pathMounted(dst) {
			if err := copyTree(dst, src); err != nil {
				return err
			}
			cmd := exec.Command("/bin/mount", "--bind", src, dst)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("bind %s: %s", name, strings.TrimSpace(string(out)))
			}
		}
		if err := appendFstabOnce(src + " " + dst + " none bind 0 0\n"); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(from, to string) error {
	cmd := exec.Command("/bin/cp", "-a", from+"/.", to+"/")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy home: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func panelAccountNames() []string {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Split(sc.Text(), ":")
		if len(p) < 4 {
			continue
		}
		uid, err := strconv.Atoi(p[2])
		if err != nil || uid < 20000 || uid > 199999 {
			continue
		}
		if !strings.HasPrefix(p[5], "/home/") {
			continue
		}
		names = append(names, p[0])
	}
	return names
}

func pathMounted(target string) bool {
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

func ensureQuotaFstab() error {
	line := quotaImage + " " + quotaMount + " ext4 loop,usrquota,grpquota 0 2\n"
	return appendFstabOnce(line)
}

func appendFstabOnce(line string) error {
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

func firstBin(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
