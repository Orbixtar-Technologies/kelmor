package brand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainsLegacyChrome(t *testing.T) {
	if got := ContainsLegacyChrome("Welcome to Kelmor Director"); got != "" {
		t.Fatalf("kelmor chrome: %q", got)
	}
	if got := ContainsLegacyChrome("<title>Server Portal</title>"); got != "Server Portal" {
		t.Fatalf("server portal: %q", got)
	}
	if got := ContainsLegacyChrome("Account Portal login"); got != "Account Portal" {
		t.Fatalf("account portal: %q", got)
	}
	if got := ContainsLegacyChrome("Hosting Panel"); got != "Hosting Panel" {
		t.Fatalf("hosting panel: %q", got)
	}
}

func TestUserVisibleChromeFiles(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{
		"portals/server/index.html",
		"portals/server/src/app.tsx",
		"portals/server/src/nav.tsx",
		"portals/server/src/dashboard.tsx",
		"portals/server/src/accounts.tsx",
		"portals/server/src/account-detail.tsx",
		"portals/server/src/pages.tsx",
		"portals/server/src/find.ts",
		"portals/account/index.html",
		"portals/account/src/app.tsx",
		"api/openapi.yaml",
		"cmd/panel-install/main.go",
		"README.md",
	}
	needles := map[string][]string{
		"portals/server/index.html":   {Director},
		"portals/server/src/app.tsx":  {Director, Product},
		"portals/server/src/nav.tsx":  {Director},
		"portals/account/index.html":  {Control},
		"portals/account/src/app.tsx": {Control, Director},
		"api/openapi.yaml":            {APITitle},
		"cmd/panel-install/main.go":   {Director, Control},
		"README.md":                   {Director, Control},
	}
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(rel, err)
		}
		text := string(body)
		if got := ContainsLegacyChrome(text); got != "" {
			t.Errorf("%s still has legacy chrome %q", rel, got)
		}
		for _, need := range needles[rel] {
			if !strings.Contains(text, need) {
				t.Errorf("%s missing %q", rel, need)
			}
		}
		if strings.HasPrefix(rel, "portals/") {
			for _, banned := range []string{"WHM", "Jupiter"} {
				if strings.Contains(text, banned) {
					t.Errorf("%s has banned chrome %q", rel, banned)
				}
			}
		}
	}
}

func TestDirectorInformationArchitecture(t *testing.T) {
	root := filepath.Join("..", "..", "portals", "server", "src")
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(e.Name(), err)
		}
		b.Write(body)
		b.WriteByte('\n')
	}
	text := b.String()
	for _, need := range []string{
		"Find",
		"Account Functions",
		"Packages",
		"Host/Service Status",
		"Jobs/Audit",
		"Import",
		"Usage",
		"Resellers",
		"List Accounts",
		"Create Account",
		"Host operations",
		"Failed jobs",
		"Quick links",
		"/api/v1/jobs?state=failed",
		"/api/v1/accounts?q=",
	} {
		if !strings.Contains(text, need) {
			t.Errorf("Director IA missing %q", need)
		}
	}
}

func TestBinaryAlias(t *testing.T) {
	if got := BinaryAlias("panel-api"); got != "kelmor-api" {
		t.Fatalf("api %q", got)
	}
	if got := BinaryAlias("panel-agent"); got != "kelmor-agent" {
		t.Fatalf("agent %q", got)
	}
	if got := BinaryAlias("unknown"); got != "unknown" {
		t.Fatalf("passthrough %q", got)
	}
}

func TestControlPlaneService(t *testing.T) {
	if !ControlPlaneService("panel-api") || !ControlPlaneService("kelmor-worker") {
		t.Fatal("zone A names")
	}
	if ControlPlaneService("panel-agent") || ControlPlaneService("kelmor-agent") {
		t.Fatal("agent is zone B")
	}
}
