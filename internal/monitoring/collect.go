package monitoring

import (
	"os"
	"path/filepath"
	"time"

	"github.com/hosting-panel/panel/internal/store"
)

type Snapshot struct {
	CollectedAt   time.Time     `json:"collected_at"`
	Accounts      []store.Usage `json:"accounts"`
	FailedJobs    int           `json:"failed_jobs"`
	CertsExpiring int           `json:"certs_expiring"`
}

func Collect(st store.Store, hostRoot string) Snapshot {
	return CollectMeasured(st, hostRoot, nil)
}

func CollectMeasured(st store.Store, hostRoot string, measure func(store.Account) *store.Usage) Snapshot {
	now := time.Now().UTC()
	snap := Snapshot{CollectedAt: now, FailedJobs: len(st.ListJobs("failed", 500))}
	horizon := now.Add(14 * 24 * time.Hour)
	for _, acc := range st.ListAccounts("", "") {
		var u store.Usage
		if measure != nil {
			if got := measure(acc); got != nil {
				u = *got
			}
		}
		if u.AccountID == "" {
			home := acc.HomePath
			if hostRoot != "" && len(acc.HomePath) > 1 {
				home = filepath.Join(hostRoot, filepath.FromSlash(acc.HomePath[1:]))
			}
			u = walkUsage(acc.ID, home, now)
		}
		st.PutUsage(&u)
		snap.Accounts = append(snap.Accounts, u)
		for _, c := range st.ListCerts(acc.ID) {
			if c.NotAfter != nil && c.NotAfter.Before(horizon) {
				snap.CertsExpiring++
			}
		}
	}
	return snap
}

func walkUsage(accountID, home string, now time.Time) store.Usage {
	u := store.Usage{AccountID: accountID, CollectedAt: now}
	_ = filepath.Walk(home, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		u.InodeCount++
		if info.Mode().IsRegular() {
			u.DiskBytes += info.Size()
		}
		return nil
	})
	return u
}
