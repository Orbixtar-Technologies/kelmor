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

func TestNeedsHostApplyForFormerDeferredKeys(t *testing.T) {
	if !NeedsHostApply([]string{"external_auth"}) || !NeedsHostApply([]string{"two_factor"}) || !NeedsHostApply([]string{"linked_nodes"}) || !NeedsHostApply([]string{"initial_quota"}) {
		t.Fatal("former PARTIAL keys must queue host apply")
	}
	if NeedsHostApply([]string{"theme"}) || NeedsHostApply([]string{"mariadb_upgrade"}) || NeedsHostApply([]string{"demo_accounts"}) {
		t.Fatal("chrome, remaining deferred, and demo-mode keys must stay local")
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

func TestStorePrefersControlAndCoercesLegacyTypes(t *testing.T) {
	state := t.TempDir()
	legacy := []byte(`{"values":{"tweak_settings":{"max_emails_hour":250,"allow_parked":true,"default_php":"8.3"}}}`)
	if err := os.WriteFile(filepath.Join(state, FileName), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(state)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Values["tweak_settings"]["max_emails_hour"] != "250" || loaded.Values["tweak_settings"]["allow_parked"] != "on" {
		t.Fatalf("coerced legacy: %+v", loaded)
	}
	if err := Store(state, File{Values: map[string]map[string]string{
		"tweak_settings": {"max_emails_hour": "400", "default_php": "8.4"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "control", FileName)); err != nil {
		t.Fatalf("preferred path: %v", err)
	}
	again, err := Load(state)
	if err != nil {
		t.Fatal(err)
	}
	if again.Values["tweak_settings"]["max_emails_hour"] != "400" {
		t.Fatalf("control wins over legacy: %+v", again)
	}
}
