package job

import "testing"

func TestPHPPoolVersionKeepsInstalledSelection(t *testing.T) {
	if got := phpPoolVersion(""); got != "8.3" {
		t.Fatalf("empty default %q", got)
	}
	if got := phpPoolVersion("8.5"); got != "8.5" {
		t.Fatalf("installed 8.5 remapped to %q", got)
	}
	if got := phpPoolVersion("8.4"); got != "8.4" {
		t.Fatalf("installed 8.4 remapped to %q", got)
	}
}
