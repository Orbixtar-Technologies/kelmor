package security

import (
	"testing"

	"github.com/hosting-panel/panel/internal/rbac"
)

func TestCustomerTokenNeverServerScope(t *testing.T) {
	a := rbac.Actor{
		Roles: []string{"customer_owner"},
		Capabilities: rbac.Expand([]string{"customer_owner"}, nil),
		IsServerScope: false,
		AccountIDs: []string{"acct-1"},
	}
	if a.Has(rbac.ServerFirewallWrite) {
		t.Fatal("customer token must not hit server firewall")
	}
	if a.CanAccount("acct-2") {
		t.Fatal("IDOR")
	}
}
