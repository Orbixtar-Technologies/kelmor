package dnsinventory

import (
	"testing"

	"github.com/hosting-panel/panel/internal/id"
	"github.com/hosting-panel/panel/internal/store"
)

func TestLeftoverZonesListsTerminatedAndUntiedOnly(t *testing.T) {
	st := store.NewMemory()
	live := putAccountZone(st, "shop", "shop.test", "active")
	gone := putAccountZone(st, "oldshop", "oldshop.test", "terminated")
	ending := putAccountZone(st, "leaving", "leaving.test", "terminating")
	untied := &store.DNSZone{
		ID: id.New(), AccountID: id.New(), DomainID: id.New(),
		Name: "orphan.test", Provider: "powerdns",
	}
	st.PutZone(untied)

	leftover := LeftoverZones(st)
	if names := candidateNames(leftover); !sameNames(names, []string{"leaving.test", "oldshop.test", "orphan.test"}) {
		t.Fatalf("leftover names %v", names)
	}
	if containsID(leftover, live.ID) {
		t.Fatal("active account zone must not be leftover")
	}
	if !containsID(leftover, gone.ID) || !containsID(leftover, ending.ID) || !containsID(leftover, untied.ID) {
		t.Fatalf("expected terminated, terminating, and untied zones: %+v", leftover)
	}
	for _, zone := range leftover {
		switch zone.Name {
		case "oldshop.test":
			if zone.Reason != "terminated_account" || zone.AccountUsername != "oldshop" {
				t.Fatalf("terminated leftover: %+v", zone)
			}
		case "leaving.test":
			if zone.Reason != "terminated_account" || zone.AccountUsername != "leaving" {
				t.Fatalf("terminating leftover: %+v", zone)
			}
		case "orphan.test":
			if zone.Reason != "untied_zone" || zone.AccountUsername != "" {
				t.Fatalf("untied leftover: %+v", zone)
			}
		default:
			t.Fatalf("unexpected leftover %q", zone.Name)
		}
	}
}

func TestManagedZonesListsLiveInventoryOnly(t *testing.T) {
	st := store.NewMemory()
	live := putAccountZone(st, "shop", "shop.test", "active")
	putAccountZone(st, "held", "held.test", "suspended")
	putAccountZone(st, "oldshop", "oldshop.test", "terminated")
	st.PutZone(&store.DNSZone{
		ID: id.New(), AccountID: id.New(), DomainID: id.New(),
		Name: "orphan.test", Provider: "powerdns",
	})

	managed := ManagedZones(st)
	if names := candidateNames(managed); !sameNames(names, []string{"held.test", "shop.test"}) {
		t.Fatalf("managed names %v", names)
	}
	if !containsID(managed, live.ID) {
		t.Fatal("active zone missing from managed list")
	}
	for _, zone := range managed {
		if zone.Reason != "managed" {
			t.Fatalf("managed reason: %+v", zone)
		}
	}
}

func TestEmptyInventoryIsHonest(t *testing.T) {
	st := store.NewMemory()
	if leftover := LeftoverZones(st); len(leftover) != 0 {
		t.Fatalf("empty leftover %v", leftover)
	}
	if managed := ManagedZones(st); len(managed) != 0 {
		t.Fatalf("empty managed %v", managed)
	}
}

func putAccountZone(st store.Store, username, domain, status string) *store.DNSZone {
	account := &store.Account{
		ID: id.New(), Username: username, PrimaryDomain: domain,
		Status: status, HomePath: "/home/" + username,
	}
	st.PutAccount(account)
	dom := &store.Domain{
		ID: id.New(), AccountID: account.ID, FQDN: domain, ASCII: domain,
		Type: "primary", Status: status,
	}
	st.PutDomain(dom)
	zone := &store.DNSZone{
		ID: id.New(), AccountID: account.ID, DomainID: dom.ID,
		Name: domain, Provider: "powerdns", DesiredRevision: 2,
	}
	st.PutZone(zone)
	return zone
}

func candidateNames(items []Candidate) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func containsID(items []Candidate, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
