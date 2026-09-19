package dnsinventory

import (
	"sort"

	"github.com/hosting-panel/panel/internal/store"
)

const (
	ReasonTerminated = "terminated_account"
	ReasonUntied     = "untied_zone"
	ReasonManaged    = "managed"
)

// Candidate is a leftover or managed zone the Director DNS wizards can review.
type Candidate struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	AccountID        string `json:"account_id,omitempty"`
	AccountUsername  string `json:"account_username,omitempty"`
	AccountStatus    string `json:"account_status,omitempty"`
	DomainID         string `json:"domain_id,omitempty"`
	Domain           string `json:"domain,omitempty"`
	Provider         string `json:"provider,omitempty"`
	Records          int    `json:"records"`
	DesiredRevision  int64  `json:"desired_revision"`
	ObservedRevision int64  `json:"observed_revision"`
	Reason           string `json:"reason"`
}

func LeftoverZones(st store.Store) []Candidate {
	live := liveZoneIDs(st)
	accounts := accountIndex(st)
	domains := domainIndex(st)
	out := []Candidate{}
	for _, zone := range st.ListZones("") {
		if live[zone.ID] {
			continue
		}
		account := lookupAccount(accounts, zone.AccountID)
		item := candidateFrom(zone, account, domains[zone.DomainID])
		if account == nil {
			item.Reason = ReasonUntied
		} else {
			item.Reason = ReasonTerminated
		}
		out = append(out, item)
	}
	sortCandidates(out)
	return out
}

func ManagedZones(st store.Store) []Candidate {
	accounts := accountIndex(st)
	domains := domainIndex(st)
	out := []Candidate{}
	for _, zone := range st.ListZones("") {
		account := lookupAccount(accounts, zone.AccountID)
		if account == nil || accountGone(account.Status) {
			continue
		}
		item := candidateFrom(zone, account, domains[zone.DomainID])
		item.Reason = ReasonManaged
		out = append(out, item)
	}
	sortCandidates(out)
	return out
}

func FilterByIDs(items []Candidate, zoneIDs []string) []Candidate {
	if len(zoneIDs) == 0 {
		return items
	}
	want := map[string]bool{}
	for _, id := range zoneIDs {
		if id != "" {
			want[id] = true
		}
	}
	var out []Candidate
	for _, item := range items {
		if want[item.ID] {
			out = append(out, item)
		}
	}
	return out
}

func PayloadZoneIDs(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	raw, ok := payload["zone_ids"]
	if !ok || raw == nil {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return compactIDs(values)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if id, ok := value.(string); ok && id != "" {
				out = append(out, id)
			}
		}
		return out
	default:
		return nil
	}
}

func liveZoneIDs(st store.Store) map[string]bool {
	live := map[string]bool{}
	for _, zone := range ManagedZones(st) {
		live[zone.ID] = true
	}
	return live
}

func accountIndex(st store.Store) map[string]store.Account {
	out := map[string]store.Account{}
	for _, account := range st.ListAccounts("", "") {
		out[account.ID] = account
	}
	return out
}

func domainIndex(st store.Store) map[string]store.Domain {
	out := map[string]store.Domain{}
	for _, account := range st.ListAccounts("", "") {
		for _, domain := range st.ListDomains(account.ID) {
			out[domain.ID] = domain
		}
	}
	return out
}

func lookupAccount(accounts map[string]store.Account, accountID string) *store.Account {
	account, ok := accounts[accountID]
	if !ok {
		return nil
	}
	return &account
}

func candidateFrom(zone store.DNSZone, account *store.Account, domain store.Domain) Candidate {
	item := Candidate{
		ID: zone.ID, Name: zone.Name, AccountID: zone.AccountID,
		DomainID: zone.DomainID, Domain: zone.Name, Provider: zone.Provider,
		Records: zone.RecordCount, DesiredRevision: zone.DesiredRevision,
		ObservedRevision: zone.ObservedRevision,
	}
	if account != nil {
		item.AccountUsername = account.Username
		item.AccountStatus = account.Status
	}
	if domain.ASCII != "" {
		item.Domain = domain.ASCII
	} else if domain.FQDN != "" {
		item.Domain = domain.FQDN
	}
	return item
}

func accountGone(status string) bool {
	return status == "terminated" || status == "terminating"
}

func sortCandidates(items []Candidate) {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
}

func compactIDs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
