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
	if c.Dev || installPrefix(c) != "" {
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
	if _, err := os.Stat(quotaMount); err != nil {
		return err
	}
	return nil
}

func ensureQuotaHomes() error {
	if !kernelQuotaSupported() {
		_ = os.WriteFile("/var/lib/panel/quota-unavailable", []byte("kernel built without CONFIG_QUOTA\n"), 0o644)
		return nil
	}
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
		// Do not enable the ext4 quota feature here. A quota-enabled
		// filesystem refuses to mount (ESRCH / "No such process") when
		// the running kernel cannot apply usrquota, which is common on
		// TCG/cloud kernels that still advertise CONFIG_QUOTA=y.
		cmd := exec.Command(mkfs, "-F", quotaImage)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs.ext4: %s", strings.TrimSpace(string(out)))
		}
	}
	if err := os.MkdirAll(quotaMount, 0o755); err != nil {
		return err
	}
	if !pathMounted(quotaMount) {
		if err := mountQuotaImage(); err != nil {
			_ = os.WriteFile("/var/lib/panel/quota-unavailable", []byte(err.Error()+"\n"), 0o644)
			return os.MkdirAll(quotaMount, 0o755)
		}
	}
	if err := enableKernelQuotaIfPossible(); err != nil {
		_ = os.WriteFile("/var/lib/panel/quota-unavailable", []byte(err.Error()+"\n"), 0o644)
	} else {
		_ = os.Remove("/var/lib/panel/quota-unavailable")
	}
	if err := ensureQuotaFstab(); err != nil {
		return err
	}
	return bindPanelHomes()
}

func mountQuotaImage() error {
	clearExt4QuotaFeature(quotaImage)
	cmd := exec.Command("/bin/mount", "-o", "loop", quotaImage, quotaMount)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	if out, err := cmd.CombinedOutput(); err == nil {
		return nil
	} else {
		loop := attachLoop(quotaImage)
		if loop == "" {
			return fmt.Errorf("mount loop: %s", strings.TrimSpace(string(out)))
		}
		cmd = exec.Command("/bin/mount", loop, quotaMount)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("mount %s: %s / %s", loop, strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
	}
	return nil
}

func attachLoop(image string) string {
	losetup := firstBin("/sbin/losetup", "/usr/sbin/losetup")
	if losetup == "" {
		return ""
	}
	out, err := exec.Command(losetup, "-f", "--show", image).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func clearExt4QuotaFeature(image string) {
	tune := firstBin("/sbin/tune2fs", "/usr/sbin/tune2fs")
	if tune == "" {
		return
	}
	cmd := exec.Command(tune, "-O", "^quota", image)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	_ = cmd.Run()
}

func enableKernelQuotaIfPossible() error {
	cmd := exec.Command("/bin/mount", "-o", "remount,usrquota,grpquota", quotaMount)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("usrquota remount: %s", strings.TrimSpace(string(out)))
	}
	quotaon := firstBin("/sbin/quotaon", "/usr/sbin/quotaon")
	if quotaon != "" {
		_ = exec.Command(quotaon, "-ug", quotaMount).Run()
	}
	return nil
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
	opts := "loop"
	if kernelQuotaMounted() {
		opts = "loop,usrquota,grpquota"
	}
	line := quotaImage + " " + quotaMount + " ext4 " + opts + " 0 2\n"
	return appendFstabOnce(line)
}

func kernelQuotaMounted() bool {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, " "+quotaMount+" ") && strings.Contains(line, "usrquota") {
			return true
		}
	}
	return false
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

func kernelQuotaSupported() bool {
	b, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	cfg := "/boot/config-" + strings.TrimSpace(string(b))
	raw, err := os.ReadFile(cfg)
	if err != nil {
		return false
	}
	s := string(raw)
	return strings.Contains(s, "CONFIG_QUOTA=y") || strings.Contains(s, "CONFIG_QUOTA=m")
}

func firstBin(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
