package hostconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndPasswordPolicy(t *testing.T) {
	dir := t.TempDir()
	body := `{"values":{"password_strength":{"min_length":"14","require_symbol":"on"},"demo_accounts":{"usernames":"shop, demo"},"zone_ttl":{"ttl":"7200"}}}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	policy := PasswordPolicyFrom(f)
	if policy.MinLength != 14 || !policy.RequireSymbol {
		t.Fatalf("policy %+v", policy)
	}
	if msg := policy.Check("short"); msg == "" {
		t.Fatal("short password must fail")
	}
	if msg := policy.Check("LongEnoughPass"); msg == "" {
		t.Fatal("symbol required")
	}
	if msg := policy.Check("LongEnoughPass!"); msg != "" {
		t.Fatalf("valid password rejected: %s", msg)
	}
	if !IsDemoUsername(f, "shop") || IsDemoUsername(f, "other") {
		t.Fatalf("demo usernames %v", DemoUsernames(f))
	}
	if ZoneTTL(f) != 7200 {
		t.Fatalf("ttl %d", ZoneTTL(f))
	}
}

func TestLoadMissingFile(t *testing.T) {
	f, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Values) != 0 {
		t.Fatalf("empty: %+v", f)
	}
}
