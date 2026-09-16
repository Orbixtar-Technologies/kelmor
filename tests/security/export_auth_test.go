package security

import (
	"testing"

	"github.com/hosting-panel/panel/internal/rbac"
)

func TestAuditorLacksExportCapabilities(t *testing.T) {
	a := rbac.Actor{Roles: []string{"auditor"}, Capabilities: rbac.Expand([]string{"auditor"}, nil)}
	if a.Has(rbac.AccountsExport) || a.Has(rbac.AccountsExportCredentials) {
		t.Fatal("auditor must not export accounts or credential hashes")
	}
}

func TestCustomerOwnerCanExportWithoutHashes(t *testing.T) {
	a := rbac.Actor{Roles: []string{"customer_owner"}, Capabilities: rbac.Expand([]string{"customer_owner"}, nil)}
	if !a.Has(rbac.AccountsExport) {
		t.Fatal("customer owner should export own account metadata")
	}
	if a.Has(rbac.AccountsExportCredentials) {
		t.Fatal("customer owner must not export reusable hashes")
	}
}

func TestRootOwnerCanExportCredentials(t *testing.T) {
	a := rbac.Actor{Roles: []string{"root_owner"}, Capabilities: rbac.Expand([]string{"root_owner"}, nil)}
	if !a.Has(rbac.AccountsExport) || !a.Has(rbac.AccountsExportCredentials) {
		t.Fatal("root owner should hold both export capabilities")
	}
}
