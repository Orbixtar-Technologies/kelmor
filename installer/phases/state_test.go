package phases

import (
	"path/filepath"
	"testing"
)

func TestLoadStateCreatesFreshAndResumes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-state.json")
	st, err := LoadState(path)
	if err != nil || st.InstallationID == "" {
		t.Fatalf("fresh: %v %#v", err, st)
	}
	st.Phases["preflight"] = "complete"
	st.Phases["packages"] = "failed"
	if err := st.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.InstallationID != st.InstallationID {
		t.Fatal("installation id changed")
	}
	if again.Phases["preflight"] != "complete" || again.Phases["packages"] != "failed" {
		t.Fatalf("phases: %#v", again.Phases)
	}
}

func TestAllPhasesHaveNames(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range All() {
		if p.Name() == "" {
			t.Fatal("empty phase name")
		}
		if seen[p.Name()] {
			t.Fatalf("duplicate phase %s", p.Name())
		}
		seen[p.Name()] = true
	}
	for _, need := range []string{"preflight", "system_packages", "control_plane", "web_stack", "mail", "tls", "quota_homes"} {
		if !seen[need] {
			t.Fatalf("missing phase %s in %v", need, seen)
		}
	}
}
