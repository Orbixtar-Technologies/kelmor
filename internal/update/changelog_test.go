package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkdownChangelogReadsVersionSections(t *testing.T) {
	doc := ParseMarkdownChangelog(`# Kelmor changelog

## Unreleased

- Upcoming note.

## 0.2.409

- First shipped note.
- Second shipped note.

## 0.1.0

`, "embedded")
	if len(doc.Items) != 2 {
		t.Fatalf("items: %+v", doc.Items)
	}
	if doc.Items[0].Version != "Unreleased" || doc.Items[0].Notes[0] != "Upcoming note." {
		t.Fatalf("unreleased: %+v", doc.Items[0])
	}
	if doc.Items[1].Version != "0.2.409" || len(doc.Items[1].Notes) != 2 {
		t.Fatalf("released: %+v", doc.Items[1])
	}
}

func TestEmbeddedChangelogHasCurrentShippedNotes(t *testing.T) {
	doc := EmbeddedChangelogDocument()
	item, ok := itemForVersion(doc, "0.2.409")
	if !ok || len(item.Notes) == 0 {
		t.Fatalf("embedded 0.2.409 notes missing: %+v", doc)
	}
	for _, note := range item.Notes {
		if strings.Contains(strings.ToLower(note), "lorem") {
			t.Fatalf("invented note: %s", note)
		}
	}
}

func TestBuildChangelogViewUsesFeedOverEmbeddedAndSkipsEmptyVersions(t *testing.T) {
	embedded := ParseMarkdownChangelog("## 0.2.409\n- Embedded only.\n## Unreleased\n- Binary notes.\n", "embedded")
	feed := ChangelogDocument{Items: []ChangelogItem{{
		Version: "0.2.410", Notes: []string{"Feed note."}, Source: "update-feed",
	}}}
	view := BuildChangelogView(Status{
		InstalledRelease: "0.2.409",
		AvailableRelease: "0.2.410",
		Channel:          "stable",
	}, "0.2.409", feed, embedded)
	if len(view.Items) != 2 {
		t.Fatalf("items: %+v", view.Items)
	}
	if view.Items[0].Version != "0.2.410" || view.Items[0].Status != "available" || view.Items[0].Notes[0] != "Feed note." {
		t.Fatalf("available: %+v", view.Items[0])
	}
	if view.Items[1].Version != "0.2.409" || view.Items[1].Status != "installed" || view.Items[1].Source != "embedded" {
		t.Fatalf("installed: %+v", view.Items[1])
	}

	empty := BuildChangelogView(Status{InstalledRelease: "9.9.9", Channel: "stable"}, "9.9.9")
	if len(empty.Items) != 0 {
		t.Fatalf("expected honest empty, got %+v", empty.Items)
	}
}

func TestNotesForReleaseFallsBackToUnreleasedForRunningBinary(t *testing.T) {
	doc := ParseMarkdownChangelog("## Unreleased\n- New Change Log page.\n", "embedded")
	item, ok := NotesForRelease(doc, "0.2.410", "0.2.410")
	if !ok || item.Notes[0] != "New Change Log page." {
		t.Fatalf("running fallback: ok=%v item=%+v", ok, item)
	}
	if _, found := NotesForRelease(doc, "0.2.410", "0.2.409"); found {
		t.Fatal("unreleased notes leaked to a different installed release")
	}
}

func TestParseJSONChangelogRejectsUnknownFieldsAndSanitizesNotes(t *testing.T) {
	_, err := ParseJSONChangelog([]byte(`{"items":[{"version":"1.0.0","notes":["ok"],"hack":true}]}`), "feed")
	if err == nil {
		t.Fatal("unknown field accepted")
	}
	doc, err := ParseJSONChangelog([]byte(`{"channel":"stable","items":[{"version":"1.0.0","notes":["  real note  ", "\u0007", ""]}]}`), "feed")
	if err != nil || len(doc.Items) != 1 || doc.Items[0].Notes[0] != "real note" {
		t.Fatalf("sanitized json: %+v %v", doc, err)
	}
}

func TestReadAndWriteChangelogCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-changelog.json")
	if err := WriteChangelogCache(path, ChangelogDocument{Items: []ChangelogItem{{
		Version: "1.2.3", Notes: []string{"Cached feed note."}, Source: "update-feed",
	}}}); err != nil {
		t.Fatal(err)
	}
	doc, err := ReadChangelogFile(path, "update-feed")
	if err != nil || len(doc.Items) != 1 || doc.Items[0].Notes[0] != "Cached feed note." {
		t.Fatalf("cache roundtrip: %+v %v", doc, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
