package inventory

import (
	"path"
	"strings"

	"github.com/hosting-panel/panel/internal/brand"
	"github.com/hosting-panel/panel/internal/update"
)

const (
	ClassFeed        = "feed"
	ClassInstallOnly = "install-only"
	KindRuntime      = "runtime"
	KindSchema       = "schema"
)

type Asset struct {
	Name     string
	Source   string
	Package  string
	FeedPath string
	Target   string
	Mode     uint32
	Class    string
	Kind     string
}

func RuntimeAssets() []Asset {
	assets := make([]Asset, 0, 48)
	for _, name := range []string{
		"panel-api", "panel-worker", "panel-agent", "panel-cli",
		"panel-updater", "panel-backup", "panel-install",
		"panel-smtp-policy", "panel-object-store",
	} {
		assets = append(assets, Asset{
			Name: name, Source: "dist/bin/" + name,
			Package:  "usr/local/panel/bin/" + name,
			FeedPath: "bin/" + name, Target: "bin/" + name,
			Mode: 0o755, Class: ClassFeed, Kind: KindRuntime,
		})
		alias := brand.BinaryAlias(name)
		if alias != name {
			assets = append(assets, Asset{
				Name: alias, Source: "dist/bin/" + name,
				Package:  "usr/local/panel/bin/" + alias,
				FeedPath: "bin/" + alias, Target: "bin/" + alias,
				Mode: 0o755, Class: ClassFeed, Kind: KindRuntime,
			})
		}
	}
	for _, name := range []string{
		"panel-api.service", "panel-worker.service", "panel-agent.service",
		"panel-smtp-policy.service", "panel-object-store.service",
		"pebble.service", "panel-update@.service", "panel-update.timer",
	} {
		assets = append(assets, Asset{
			Name: name, Source: "installer/phases/units/" + name,
			Package:  "etc/systemd/system/" + name,
			FeedPath: "share/systemd/" + name, Target: "share/systemd/" + name,
			Mode: 0o644, Class: ClassFeed, Kind: KindRuntime,
		})
	}
	assets = append(assets,
		Asset{
			Name: "portals/server", Source: "dist/share/portals/server",
			Package:  "usr/local/panel/share/portals/server",
			FeedPath: "share/portals/server", Target: "share/portals/server/index.html",
			Mode: 0o644, Class: ClassFeed, Kind: KindRuntime,
		},
		Asset{
			Name: "portals/account", Source: "dist/share/portals/account",
			Package:  "usr/local/panel/share/portals/account",
			FeedPath: "share/portals/account", Target: "share/portals/account/index.html",
			Mode: 0o644, Class: ClassFeed, Kind: KindRuntime,
		},
		Asset{
			Name: "migrations", Source: "db/migrations",
			Package:  "usr/local/panel/share/migrations",
			FeedPath: "share/migrations", Target: "share/migrations/000001_extensions.sql",
			Mode: 0o644, Class: ClassFeed, Kind: KindSchema,
		},
		Asset{
			Name: "templates", Package: "usr/local/panel/share/templates",
			Class: ClassInstallOnly, Kind: KindRuntime,
		},
		Asset{
			Name: "testdata/cpanel-acme42", Source: "testdata/cpanel-acme42",
			Package: "usr/local/panel/share/testdata/cpanel-acme42",
			Class:   ClassInstallOnly, Kind: KindRuntime,
		},
		Asset{
			Name: "scripts/live-e2e.sh", Source: "scripts/live-e2e.sh",
			Package: "usr/local/panel/share/scripts/live-e2e.sh",
			Mode:    0o755, Class: ClassInstallOnly, Kind: KindRuntime,
		},
		Asset{
			Name: "scripts/cli-mvp.sh", Source: "scripts/cli-mvp.sh",
			Package: "usr/local/panel/share/scripts/cli-mvp.sh",
			Mode:    0o755, Class: ClassInstallOnly, Kind: KindRuntime,
		},
		Asset{
			Name: "scripts/fresh-provision-smoke.sh", Source: "scripts/fresh-provision-smoke.sh",
			Package: "usr/local/panel/share/scripts/fresh-provision-smoke.sh",
			Mode:    0o755, Class: ClassInstallOnly, Kind: KindRuntime,
		},
		Asset{
			Name: "docs/README.md", Source: "README.md",
			Package: "usr/share/doc/hosting-panel/README.md",
			Mode:    0o644, Class: ClassInstallOnly, Kind: KindRuntime,
		},
	)
	return assets
}

func FeedAssets() []Asset {
	out := make([]Asset, 0)
	for _, asset := range RuntimeAssets() {
		if asset.Class == ClassFeed {
			out = append(out, asset)
		}
	}
	return out
}

func InstallOnlyAssets() []Asset {
	out := make([]Asset, 0)
	for _, asset := range RuntimeAssets() {
		if asset.Class == ClassInstallOnly {
			out = append(out, asset)
		}
	}
	return out
}

func MapFeedTarget(rel string) (string, bool) {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "." || rel == "manifest.json" {
		return "", false
	}
	if update.AllowedTarget(rel) {
		return rel, true
	}
	for _, asset := range FeedAssets() {
		if rel == asset.FeedPath || strings.HasPrefix(rel, strings.TrimRight(asset.FeedPath, "/")+"/") {
			if update.AllowedTarget(rel) {
				return rel, true
			}
			return asset.Target, update.AllowedTarget(asset.Target)
		}
	}
	return "", false
}
