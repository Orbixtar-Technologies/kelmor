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
		"portals/account/index.html",
		"portals/account/src/app.tsx",
		"api/openapi.yaml",
		"cmd/panel-install/main.go",
		"README.md",
	}
	needles := map[string][]string{
		"portals/server/index.html":   {Director},
		"portals/server/src/app.tsx":  {Director, Product},
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
