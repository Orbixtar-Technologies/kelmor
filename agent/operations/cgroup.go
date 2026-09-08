package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hosting-panel/panel/internal/pkg/validate"
	"golang.org/x/sys/unix"
)

const panelCgroupRoot = "/sys/fs/cgroup/panel.accounts"

func (h *Host) applyCgroupLimits(username string, cpuPercent int, memoryBytes int64, tasks, ioWeight, iops int) error {
	if err := validate.Username(username); err != nil {
		return err
	}
	h.persistCgroupSpec(username, cpuPercent, memoryBytes, tasks, ioWeight, iops)
	if !h.live() {
		return nil
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return nil
	}
	if err := os.MkdirAll(panelCgroupRoot, 0o755); err != nil {
		return fmt.Errorf("cgroup parent: %w", err)
	}
	_ = os.WriteFile(filepath.Join(panelCgroupRoot, "cgroup.subtree_control"), []byte("+cpu +memory +pids +io\n"), 0o644)
	dir := filepath.Join(panelCgroupRoot, username)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cgroup %s: %w", username, err)
	}
	if memoryBytes > 0 {
		if err := os.WriteFile(filepath.Join(dir, "memory.max"), []byte(strconv.FormatInt(memoryBytes, 10)+"\n"), 0o644); err != nil {
			return fmt.Errorf("memory.max: %w", err)
		}
	}
	if cpuPercent > 0 {
		quota := int64(cpuPercent) * 1000
		if err := os.WriteFile(filepath.Join(dir, "cpu.max"), []byte(fmt.Sprintf("%d 100000\n", quota)), 0o644); err != nil {
			return fmt.Errorf("cpu.max: %w", err)
		}
	}
	if tasks > 0 {
		if err := os.WriteFile(filepath.Join(dir, "pids.max"), []byte(strconv.Itoa(tasks)+"\n"), 0o644); err != nil {
			return fmt.Errorf("pids.max: %w", err)
		}
	}
	h.applyCgroupIO(dir, username, ioWeight, iops)
	attachUserProcesses(username, dir)
	return nil
}

func (h *Host) persistCgroupSpec(username string, cpuPercent int, memoryBytes int64, tasks, ioWeight, iops int) {
	body := fmt.Sprintf("cpu_percent=%d\nmemory_bytes=%d\nprocess_limit=%d\nio_weight=%d\niops=%d\n",
		cpuPercent, memoryBytes, tasks, ioWeight, iops)
	_, _ = h.ApplyFile("/var/lib/panel/cgroup/"+username, []byte(body), 0o644)
}

func (h *Host) applyCgroupIO(dir, username string, ioWeight, iops int) {
	if ioWeight > 0 {
		if ioWeight > 10000 {
			ioWeight = 10000
		}
		_ = os.WriteFile(filepath.Join(dir, "io.weight"), []byte(fmt.Sprintf("default %d\n", ioWeight)), 0o644)
	}
	if iops < 1 {
		return
	}
	maj, min, ok := backingDev("/home/" + username)
	if !ok {
		maj, min, ok = backingDev("/home")
	}
	if !ok {
		return
	}
	line := fmt.Sprintf("%d:%d riops=%d wiops=%d\n", maj, min, iops, iops)
	_ = os.WriteFile(filepath.Join(dir, "io.max"), []byte(line), 0o644)
}

func backingDev(path string) (maj, min uint32, ok bool) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, 0, false
	}
	return unix.Major(st.Dev), unix.Minor(st.Dev), true
}

func attachPID(dir string, pid int) {
	if pid <= 1 || dir == "" {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

func attachUserProcesses(username, dir string) {
	u, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return
	}
	uid := ""
	for _, line := range strings.Split(string(u), "\n") {
		f := strings.Split(line, ":")
		if len(f) > 2 && f[0] == username {
			uid = f[2]
			break
		}
	}
	if uid == "" {
		return
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		st, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(st), "\n") {
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) > 1 && fields[1] == uid {
					attachPID(dir, pid)
				}
				break
			}
		}
	}
}

func accountCgroupDir(username string) string {
	return filepath.Join(panelCgroupRoot, username)
}

func (h *Host) freezeAccount(username string, freeze bool) error {
	if err := validate.Username(username); err != nil {
		return err
	}
	if !h.live() {
		return nil
	}
	v := "0\n"
	if freeze {
		v = "1\n"
	}
	path := filepath.Join(accountCgroupDir(username), "cgroup.freeze")
	if err := os.WriteFile(path, []byte(v), 0o644); err != nil && freeze {
		return fmt.Errorf("cgroup.freeze: %w", err)
	}
	return nil
}
