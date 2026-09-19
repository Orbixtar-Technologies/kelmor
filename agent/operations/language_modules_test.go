package operations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateLanguageModuleName(t *testing.T) {
	cases := []struct {
		kind, name string
		ok         bool
	}{
		{"perl", "JSON::XS", true},
		{"perl", "DBI", true},
		{"pear", "Archive_Tar", true},
		{"pecl", "redis", true},
		{"ruby", "nokogiri", true},
		{"perl", "JSON::XS; rm -rf /", false},
		{"pecl", "../x", false},
		{"ruby", "gem name", false},
		{"pear", "", false},
		{"unknown", "redis", false},
	}
	for _, tc := range cases {
		err := validateLanguageModuleName(tc.kind, tc.name)
		if tc.ok && err != nil {
			t.Fatalf("%s %s: unexpected error %v", tc.kind, tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s %s: expected rejection", tc.kind, tc.name)
		}
	}
}

func TestInstallLanguageModuleStagesAndLists(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.installLanguageModule("perl", "JSON::XS;id"); err == nil {
		t.Fatal("hostile perl name must be rejected")
	}
	res, err := h.installLanguageModule("perl", "JSON::XS")
	if err != nil || res.ObservedState != "staged" {
		t.Fatalf("staged perl: %v %#v", err, res)
	}
	if _, err := h.installLanguageModule("pecl", "redis"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.installLanguageModule("pear", "Archive_Tar"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.installLanguageModule("ruby", "rake"); err != nil {
		t.Fatal(err)
	}
	perl, err := h.listLanguageModules("perl")
	if err != nil || len(perl) != 1 || perl[0].Name != "JSON::XS" {
		t.Fatalf("perl list: %v %#v", err, perl)
	}
	all, err := h.listLanguageModules("")
	if err != nil || len(all) != 4 {
		t.Fatalf("all modules: %v %#v", err, all)
	}
	raw, err := os.ReadFile(filepath.Join(h.Root, "etc/panel/language-modules.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file languageModuleFile
	if err := json.Unmarshal(raw, &file); err != nil || len(file.Items) != 4 {
		t.Fatalf("state file: %s %v", raw, err)
	}
}

func TestPerlAptPackage(t *testing.T) {
	if got := perlAptPackage("JSON::XS"); got != "libjson-xs-perl" {
		t.Fatalf("got %s", got)
	}
}
