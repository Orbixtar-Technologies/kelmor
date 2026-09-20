package update

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed changelog.md
var EmbeddedChangelog string

const (
	maxChangelogBytes = 64 << 10
	maxNoteCount      = 40
	maxNoteRunes      = 400
	unreleasedVersion = "Unreleased"
)

// ChangelogDocument is a display-only collection of release notes.
// Feed copies are unsigned extras next to manifest.json and must not
// influence install or signature decisions.
type ChangelogDocument struct {
	Channel string          `json:"channel,omitempty"`
	Items   []ChangelogItem `json:"items"`
}

// ChangelogItem is one version's operator-facing notes.
type ChangelogItem struct {
	Version string   `json:"version"`
	Title   string   `json:"title,omitempty"`
	Notes   []string `json:"notes"`
	Source  string   `json:"source,omitempty"`
}

// ChangelogView is the Director Change Log payload.
type ChangelogView struct {
	InstalledRelease string              `json:"installed_release"`
	AvailableRelease string              `json:"available_release,omitempty"`
	RunningRelease   string              `json:"running_release,omitempty"`
	Channel          string              `json:"channel"`
	Items            []ChangelogViewItem `json:"items"`
}

// ChangelogViewItem is one installed or available release with notes.
type ChangelogViewItem struct {
	Version string   `json:"version"`
	Status  string   `json:"status"`
	Source  string   `json:"source"`
	Title   string   `json:"title,omitempty"`
	Notes   []string `json:"notes"`
}

type versionRef struct {
	Version string
	Status  string
}

// ParseMarkdownChangelog reads Keep-a-Changelog style `## version` sections.
func ParseMarkdownChangelog(raw, source string) ChangelogDocument {
	doc := ChangelogDocument{}
	if strings.TrimSpace(raw) == "" {
		return doc
	}
	var current *ChangelogItem
	flush := func() {
		if current == nil {
			return
		}
		current.Notes = sanitizeNotes(current.Notes)
		if current.Version != "" && len(current.Notes) > 0 {
			current.Source = source
			doc.Items = append(doc.Items, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			heading = strings.TrimPrefix(heading, "v")
			current = &ChangelogItem{Version: heading}
			continue
		}
		if current == nil {
			continue
		}
		if note, ok := markdownNote(trimmed); ok {
			current.Notes = append(current.Notes, note)
		}
	}
	flush()
	return doc
}

// ParseJSONChangelog decodes the unsigned feed changelog sidecar.
func ParseJSONChangelog(raw []byte, source string) (ChangelogDocument, error) {
	if len(raw) > maxChangelogBytes {
		return ChangelogDocument{}, fmt.Errorf("changelog exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var doc ChangelogDocument
	if err := decoder.Decode(&doc); err != nil {
		return ChangelogDocument{}, fmt.Errorf("decode changelog: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ChangelogDocument{}, fmt.Errorf("decode changelog: trailing data")
	}
	clean := ChangelogDocument{Channel: strings.TrimSpace(doc.Channel)}
	for _, item := range doc.Items {
		version := strings.TrimSpace(strings.TrimPrefix(item.Version, "v"))
		notes := sanitizeNotes(item.Notes)
		if version == "" || len(notes) == 0 {
			continue
		}
		clean.Items = append(clean.Items, ChangelogItem{
			Version: version,
			Title:   sanitizeTitle(item.Title),
			Notes:   notes,
			Source:  source,
		})
	}
	return clean, nil
}

// EmbeddedChangelogDocument returns notes compiled into this binary.
func EmbeddedChangelogDocument() ChangelogDocument {
	return ParseMarkdownChangelog(EmbeddedChangelog, "embedded")
}

// NotesForRelease returns sanitized notes for one version, falling back to
// Unreleased when that version is the running binary identity.
func NotesForRelease(doc ChangelogDocument, version, running string) (ChangelogItem, bool) {
	if item, ok := itemForVersion(doc, version); ok {
		return item, true
	}
	if running != "" && version == running {
		return itemForVersion(doc, unreleasedVersion)
	}
	return ChangelogItem{}, false
}

// BuildChangelogView keeps only installed and available versions.
func BuildChangelogView(status Status, running string, docs ...ChangelogDocument) ChangelogView {
	view := ChangelogView{
		InstalledRelease: strings.TrimSpace(status.InstalledRelease),
		AvailableRelease: strings.TrimSpace(status.AvailableRelease),
		RunningRelease:   strings.TrimSpace(running),
		Channel:          strings.TrimSpace(status.Channel),
		Items:            []ChangelogViewItem{},
	}
	seen := map[string]bool{}
	for _, ref := range changelogVersions(status) {
		if ref.Version == "" || seen[ref.Version] {
			continue
		}
		seen[ref.Version] = true
		item, source, ok := firstNotes(docs, ref.Version, running)
		if !ok {
			continue
		}
		view.Items = append(view.Items, ChangelogViewItem{
			Version: ref.Version,
			Status:  ref.Status,
			Source:  source,
			Title:   item.Title,
			Notes:   item.Notes,
		})
	}
	return view
}

func changelogVersions(status Status) []versionRef {
	installed := strings.TrimSpace(status.InstalledRelease)
	available := strings.TrimSpace(status.AvailableRelease)
	refs := []versionRef{}
	if available != "" && available != installed {
		refs = append(refs, versionRef{Version: available, Status: "available"})
	}
	if installed != "" {
		refs = append(refs, versionRef{Version: installed, Status: "installed"})
	}
	return refs
}

func firstNotes(docs []ChangelogDocument, version, running string) (ChangelogItem, string, bool) {
	for _, doc := range docs {
		if item, ok := NotesForRelease(doc, version, running); ok {
			source := item.Source
			if source == "" {
				source = "embedded"
			}
			return item, source, true
		}
	}
	return ChangelogItem{}, "", false
}

func itemForVersion(doc ChangelogDocument, version string) (ChangelogItem, bool) {
	want := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if want == "" {
		return ChangelogItem{}, false
	}
	for _, item := range doc.Items {
		if strings.EqualFold(strings.TrimSpace(item.Version), want) {
			if len(item.Notes) == 0 {
				return ChangelogItem{}, false
			}
			return item, true
		}
	}
	return ChangelogItem{}, false
}

func markdownNote(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "- "):
		return strings.TrimSpace(strings.TrimPrefix(line, "- ")), true
	case strings.HasPrefix(line, "* "):
		return strings.TrimSpace(strings.TrimPrefix(line, "* ")), true
	default:
		return "", false
	}
}

func sanitizeNotes(notes []string) []string {
	out := make([]string, 0, len(notes))
	for _, note := range notes {
		clean := sanitizeTitle(note)
		if clean == "" {
			continue
		}
		out = append(out, clean)
		if len(out) >= maxNoteCount {
			break
		}
	}
	return out
}

func sanitizeTitle(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		if r == '\uFFFD' || (unicode.IsControl(r) && r != '\t') {
			continue
		}
		builder.WriteRune(r)
		if utf8.RuneCountInString(builder.String()) >= maxNoteRunes {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

func ChangelogCachePath(statusPath, installRoot string) string {
	if statusPath != "" {
		return filepath.Join(filepath.Dir(statusPath), "update-changelog.json")
	}
	if installRoot != "" {
		return filepath.Join(installRoot, "var/lib/panel/update-changelog.json")
	}
	return ""
}

func FeedChangelogPath(installRoot, channel string) string {
	channel = strings.TrimSpace(channel)
	if !validRelativePath(channel) || strings.Contains(channel, "/") {
		return ""
	}
	if installRoot == "" {
		return filepath.Join("/usr/local/panel/share/updates", channel, "changelog.json")
	}
	return filepath.Join(installRoot, "usr/local/panel/share/updates", channel, "changelog.json")
}

func ReadChangelogFile(path, source string) (ChangelogDocument, error) {
	if path == "" {
		return ChangelogDocument{}, fmt.Errorf("changelog path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ChangelogDocument{}, err
	}
	if len(raw) > maxChangelogBytes {
		return ChangelogDocument{}, fmt.Errorf("changelog exceeds size limit")
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ChangelogDocument{}, nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return ParseJSONChangelog(trimmed, source)
	}
	return ParseMarkdownChangelog(string(trimmed), source), nil
}

func WriteChangelogCache(path string, doc ChangelogDocument) error {
	if path == "" {
		return fmt.Errorf("changelog path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-changelog.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
