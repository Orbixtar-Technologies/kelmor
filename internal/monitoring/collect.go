package monitoring

import (
	"os"
	"path/filepath"
	"time"

	"github.com/hosting-panel/panel/internal/store"
)

type Snapshot struct {
	CollectedAt   time.Time     `json:"collected_at"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Accounts      []store.Usage `json:"accounts"`
	FailedJobs    int           `json:"failed_jobs"`
	CertsExpiring int           `json:"certs_expiring"`
}

func Collect(st store.Store, hostRoot string) Snapshot {
	return CollectLatest(st, time.Now().UTC())
}

func CollectLatest(st store.Store, now time.Time) Snapshot {
	snap := Snapshot{CollectedAt: now, GeneratedAt: now, FailedJobs: len(st.ListJobs("failed", 500))}
	horizon := now.Add(14 * 24 * time.Hour)
	page := st.ListAccountsPage("", "", "", store.MaxInventoryLimit)
	for _, acc := range page.Items {
		if usage := st.GetUsage(acc.ID); usage != nil {
			sample := *usage
			if !sample.CollectedAt.IsZero() {
				sample.StaleSeconds = int(now.Sub(sample.CollectedAt).Seconds())
			}
			snap.Accounts = append(snap.Accounts, sample)
		}
	}
	for _, cert := range st.ListDueCertificates(horizon, store.CertRenewalBatch) {
		_ = cert
		snap.CertsExpiring++
	}
	return snap
}

func CollectMeasured(st store.Store, hostRoot string, measure func(store.Account) *store.Usage) Snapshot {
	now := time.Now().UTC()
	snap := Snapshot{CollectedAt: now, GeneratedAt: now, FailedJobs: len(st.ListJobs("failed", 500))}
	horizon := now.Add(14 * 24 * time.Hour)
	page := st.ListAccountsPage("", "", "", store.MaxInventoryLimit)
	for _, acc := range page.Items {
		var u store.Usage
		if measure != nil {
			if got := measure(acc); got != nil {
				u = *got
			}
		}
		if u.AccountID == "" {
			if stored := st.GetUsage(acc.ID); stored != nil {
				u = *stored
			} else {
				home := acc.HomePath
				if hostRoot != "" && len(acc.HomePath) > 1 {
					home = filepath.Join(hostRoot, filepath.FromSlash(acc.HomePath[1:]))
				}
				u = walkUsage(acc.ID, home, now)
			}
		}
		st.PutUsage(&u)
		if !u.CollectedAt.IsZero() {
			u.StaleSeconds = int(now.Sub(u.CollectedAt).Seconds())
		}
		snap.Accounts = append(snap.Accounts, u)
	}
	snap.CertsExpiring = len(st.ListDueCertificates(horizon, store.CertRenewalBatch))
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
		if u.InodeCount >= store.MaxWalkInodes {
			return filepath.SkipAll
		}
		return nil
	})
	return u
}
