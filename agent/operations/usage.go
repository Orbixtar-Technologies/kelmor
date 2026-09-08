package operations

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/pkg/validate"
)

type AccountUsage struct {
	Username       string `json:"username"`
	Home           string `json:"home"`
	DiskBytes      int64  `json:"disk_bytes"`
	InodeCount     int64  `json:"inode_count"`
	ProcessCount   int64  `json:"process_count"`
	MemoryBytes    int64  `json:"memory_bytes"`
	BandwidthBytes int64  `json:"bandwidth_bytes"`
}

func (h *Host) measureAccountUsage(username, home string) (AccountUsage, error) {
	if err := validate.Username(username); err != nil {
		return AccountUsage{}, err
	}
	if home == "" {
		home = "/home/" + username
	}
	root, err := h.resolve(home)
	if err != nil {
		return AccountUsage{}, err
	}
	u := AccountUsage{Username: username, Home: home}
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil {
			return nil
		}
		u.InodeCount++
		if info.Mode().IsRegular() {
			u.DiskBytes += info.Size()
		}
		return nil
	})
	if h.live() {
		u.ProcessCount, u.MemoryBytes = processUsageFor(username)
	}
	u.BandwidthBytes = h.sumNginxBandwidth(h.websiteIDsForAccount(username), time.Now().UTC())
	h.persistBandwidthTotal(username, time.Now().UTC(), u.BandwidthBytes)
	return u, nil
}

func processUsageFor(username string) (procs, rss int64) {
	u, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return 0, 0
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
		return 0, 0
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0
	}
	for _, e := range ents {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		st, err := os.ReadFile(filepath.Join("/proc", e.Name(), "status"))
		if err != nil {
			continue
		}
		var gotUID string
		var rssKb int64
		for _, line := range strings.Split(string(st), "\n") {
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					gotUID = fields[1]
				}
			}
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					rssKb, _ = strconv.ParseInt(fields[1], 10, 64)
				}
			}
		}
		if gotUID == uid {
			procs++
			rss += rssKb * 1024
		}
	}
	return procs, rss
}
