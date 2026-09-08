package limits

import (
	"fmt"

	"github.com/hosting-panel/panel/internal/store"
)

type Check struct {
	Kind  string
	Limit int64
	Used  int64
}

func (c Check) Error() string {
	return fmt.Sprintf("%s limit exceeded (%d/%d)", c.Kind, c.Used, c.Limit)
}

func Enforce(used, limit int, kind string) error {
	if limit <= 0 {
		return nil
	}
	if used >= limit {
		return Check{Kind: kind, Limit: int64(limit), Used: int64(used)}
	}
	return nil
}

func DiskWouldExceed(used, incoming, limit int64) error {
	if limit <= 0 {
		return nil
	}
	if used+incoming > limit {
		return Check{Kind: "disk_bytes", Limit: limit, Used: used + incoming}
	}
	return nil
}

func CountDomains(st store.Store, accountID, typ string) int {
	n := 0
	for _, d := range st.ListDomains(accountID) {
		if domainMatchesKind(d.Type, typ) {
			n++
		}
	}
	return n
}

func DomainKind(typ string) string {
	switch typ {
	case "subdomain":
		return "subdomains"
	case "alias":
		return "alias_domains"
	default:
		return "domains"
	}
}

func DomainLimit(pkg *store.Package, typ string) int {
	if pkg == nil {
		return 0
	}
	switch typ {
	case "subdomain":
		return pkg.Subdomains
	case "alias":
		return pkg.AliasDomains
	default:
		return pkg.Domains
	}
}

func domainMatchesKind(have, wantKind string) bool {
	switch wantKind {
	case "subdomains":
		return have == "subdomain"
	case "alias_domains":
		return have == "alias"
	default:
		return have == "primary" || have == "addon" || have == ""
	}
}
