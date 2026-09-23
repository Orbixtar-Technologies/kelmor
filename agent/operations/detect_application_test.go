package operations

import "testing"

func TestDetectApplicationNodeStartAndSPA(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if _, err := h.ApplyFile("/home/acme/app/package.json", []byte(`{"scripts":{"start":"node index.js"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := h.DetectApplication("/home/acme/app")
	if err != nil || got.Runtime != "node" || got.StartCmd != "/usr/bin/npm start" {
		t.Fatalf("%+v %v", got, err)
	}

	h = &Host{Root: t.TempDir()}
	if _, err := h.ApplyFile("/home/acme/spa/package.json", []byte(`{"scripts":{"build":"vite build"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ApplyFile("/home/acme/spa/pnpm-lock.yaml", []byte("lockfileVersion: '9.0'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = h.DetectApplication("/home/acme/spa")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "spa" || got.Manager != "pnpm" || got.StartCmd != "/usr/bin/node "+panelStaticServerName {
		t.Fatalf("spa detect: %+v", got)
	}
	if got.BuildCmd == "" {
		t.Fatal("expected build command")
	}
}
