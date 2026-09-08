package dns

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hosting-panel/panel/internal/store"
)

func ZoneFile(zone store.DNSZone, records []store.DNSRecord, serial int64) string {
	origin := strings.TrimSuffix(zone.Name, ".") + "."
	var b strings.Builder
	fmt.Fprintf(&b, "$ORIGIN %s\n$TTL 3600\n", origin)
	fmt.Fprintf(&b, "@ IN SOA ns1.%s hostmaster.%s (%d 7200 3600 1209600 3600)\n", origin, origin, serial)
	fmt.Fprintf(&b, "@ IN NS ns1.%s\n", origin)
	recs := append([]store.DNSRecord(nil), records...)
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Name == recs[j].Name {
			return recs[i].Type < recs[j].Type
		}
		return recs[i].Name < recs[j].Name
	})
	for _, r := range recs {
		name := r.Name
		if name == "" {
			name = "@"
		}
		prio := ""
		if r.Priority != nil && (r.Type == "MX" || r.Type == "SRV") {
			prio = fmt.Sprintf("%d ", *r.Priority)
		}
		content := r.Content
		switch r.Type {
		case "MX", "CNAME", "NS", "SRV":
			if content != "" && !strings.HasSuffix(content, ".") && !strings.Contains(content, ":") {
				content += "."
			}
		}
		fmt.Fprintf(&b, "%s %d IN %s %s%s\n", name, r.TTL, r.Type, prio, content)
	}
	return b.String()
}
