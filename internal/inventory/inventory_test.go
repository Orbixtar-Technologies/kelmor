package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/update"
)

func TestRuntimeInventoryCoversRequiredAssets(t *testing.T) {
	assets := RuntimeAssets()
	if len(assets) == 0 {
		t.Fatal("runtime inventory is empty")
	}
	need := []string{
		"panel-api", "panel-worker", "panel-agent", "panel-cli",
		"panel-updater", "panel-backup", "panel-install",
		"panel-smtp-policy", "panel-object-store",
		"kelmor-api", "kelmor-install",
		"panel-api.service", "panel-update.timer",
		"portals/server", "portals/account",
		"migrations", "templates", "policy",
	}
	joined := ""
	for _, asset := range assets {
		joined += asset.Name + " " + asset.Source + " " + asset.Package + " " + asset.FeedPath + " " + asset.Target + "\n"
		if asset.Name == "" || asset.Class == "" {
			t.Fatalf("asset missing identity: %+v", asset)
		}
		if asset.Class == ClassFeed && (asset.FeedPath == "" || asset.Target == "") {
			t.Fatalf("feed asset missing paths: %+v", asset)
		}
		if asset.Class == ClassInstallOnly && asset.Package == "" {
			t.Fatalf("install-only asset missing package path: %+v", asset)
		}
	}
	for _, name := range need {
		if !strings.Contains(joined, name) {
			t.Fatalf("inventory missing %q\n%s", name, joined)
		}
	}
}

func TestFeedTargetsHaveUpdaterParity(t *testing.T) {
	for _, asset := range FeedAssets() {
		if !update.AllowedTarget(asset.Target) {
			t.Fatalf("feed target %q is not allowed by the updater", asset.Target)
		}
	}
}

func TestPackagingAndFeedScriptsUseInventory(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	build, err := os.ReadFile(filepath.Join(root, "packaging/debian/build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(root, "scripts/bootstrap-update-feed.sh"))
	if err != nil {
		t.Fatal(err)
	}
	mapper, err := os.ReadFile(filepath.Join(root, "scripts/build-update-feed/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{build, feed} {
		if !strings.Contains(string(body), "release-inventory") {
			t.Fatal("packaging or feed script still hardcodes copy lists")
		}
	}
	if !strings.Contains(string(mapper), "inventory.") {
		t.Fatal("update-feed mapper does not use the runtime inventory")
	}
}
