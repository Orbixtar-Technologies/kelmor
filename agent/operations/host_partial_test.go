package operations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLinkedNodesAndQuotaPolicy(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := h.writeLinkedNodes([]string{"https://peer.example.test:2087", "https://peer.example.test:2087"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/linked-nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file linkedNodesFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Nodes) != 1 || file.Nodes[0] != "https://peer.example.test:2087" {
		t.Fatalf("nodes: %+v", file)
	}
	if err := h.writeInitialQuotaPolicy(2<<30, true); err != nil {
		t.Fatal(err)
	}
	status, err := h.probeQuotaStatus()
	if err != nil {
		t.Fatal(err)
	}
	row := status.(map[string]any)
	if row["policy_bytes"].(int64) != 2<<30 || row["enforce"] != true {
		t.Fatalf("status: %v", row)
	}
}

func TestWriteExternalAuthRejectsBadProvider(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := h.writeExternalAuth(ExternalAuthSpec{Provider: "saml"}); err == nil {
		t.Fatal("saml must be rejected until a host stack exists")
	}
	if err := h.writeExternalAuth(ExternalAuthSpec{Provider: "ldap", LDAPURL: "ldap://directory.example.test", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.Root, "etc/panel/external-auth.json")); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxLDAPBindRecordsAttempt(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	res, err := h.verifyLDAPBind("ldap://directory.example.test", "uid=admin,ou=people,dc=test", "secret")
	if err != nil || !res.OK {
		t.Fatalf("sandbox bind: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/ldap-bind-attempts")); err != nil {
		t.Fatal(err)
	}
}
