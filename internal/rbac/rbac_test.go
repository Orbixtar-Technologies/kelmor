package rbac

import "testing"

func TestResellerDoesNotGetFirewall(t *testing.T) {
	a := Actor{Roles: []string{"reseller"}, Capabilities: Expand([]string{"reseller"}, nil)}
	if a.Has(ServerFirewallWrite) {
		t.Fatal("reseller must not write firewall")
	}
	if !a.Has(AccountsCreate) {
		t.Fatal("reseller should create accounts")
	}
}

func TestCustomerBoundary(t *testing.T) {
	a := Actor{Roles: []string{"customer_owner"}, AccountIDs: []string{"a1"}, Capabilities: Expand([]string{"customer_owner"}, nil)}
	if a.CanAccount("a2") {
		t.Fatal("cross-account")
	}
	if !a.CanAccount("a1") {
		t.Fatal("own account")
	}
}
