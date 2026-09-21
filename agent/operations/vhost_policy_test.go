package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListMailQueueNeverErrors(t *testing.T) {
	host := &Host{Root: t.TempDir()}
	result := host.listMailQueue()
	if result.Items == nil {
		t.Fatal("items must be a slice")
	}
	if len(result.Items) != 0 {
		t.Fatalf("empty host queue %v", result)
	}
	if result.Partial {
		t.Fatalf("sandbox must not report a live read failure: %+v", result)
	}
	if strings.Contains(result.Message, "Could not read") {
		t.Fatalf("sandbox empty must not be a read failure: %+v", result)
	}
}

func TestListGitReposDiscoversDotGit(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	home := "/home/shop"
	if err := os.MkdirAll(filepath.Join(root, "home/shop/app/.git"), 0o755); err != nil {
		t.Fatal(err)
	}
	repos, err := host.listGitRepos(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Path != "/app" {
		t.Fatalf("repos %+v", repos)
	}
}

func TestListImagesEmptyRoot(t *testing.T) {
	host := &Host{Root: t.TempDir()}
	images, err := host.listImages("/home/missing/public_html")
	if err != nil {
		t.Fatal(err)
	}
	if images == nil || len(images) != 0 {
		t.Fatalf("images %+v", images)
	}
}

func TestReadWriteVhostPolicy(t *testing.T) {
	host := &Host{Root: t.TempDir()}
	written, err := host.writeVhostPolicy(VhostPolicy{
		WebsiteID: "web1",
		Redirects: []VhostRedirect{{ID: "r1", Source: "/old", Target: "https://shop.test/new", Status: 301}},
		Hotlink:   VhostHotlink{Enabled: true, AllowDirect: true, Extensions: []string{"jpg"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if written.WebsiteID != "web1" || len(written.Redirects) != 1 {
		t.Fatalf("written %+v", written)
	}
	read, err := host.readVhostPolicy("web1")
	if err != nil {
		t.Fatal(err)
	}
	if !read.Hotlink.Enabled || read.Redirects[0].Source != "/old" {
		t.Fatalf("read %+v", read)
	}
}
