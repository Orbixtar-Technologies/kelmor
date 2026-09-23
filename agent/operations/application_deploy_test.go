package operations

import (
	"os"
	"strings"
	"testing"
)

func TestDeployApplicationDetectsAndPreservesSource(t *testing.T) {
	for _, tc := range []struct{ runtime, file, content, command string }{
		{"node", "server.js", "// customer code", "/usr/bin/node server.js"},
		{"node", "package.json", `{"scripts":{"start":"node index.js"}}`, "/usr/bin/npm start"},
		{"python", "app.py", "# customer code", "/usr/bin/python3 app.py"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			h := &Host{Root: t.TempDir()}
			path := "/home/acme/app/" + tc.file
			if _, err := h.ApplyFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			result, err := h.deployApplication("site-1", "acme", tc.runtime, "/home/acme/app", "")
			if err != nil || result.ObservedState != "configured" {
				t.Fatalf("%+v %v", result, err)
			}
			body, err := h.readManaged("/etc/systemd/system/panel-app-site-1.service", 10000)
			if err != nil || !strings.Contains(string(body), "ExecStart="+tc.command) || !strings.Contains(string(body), "StartLimitBurst=3") {
				t.Fatalf("%s %v", body, err)
			}
			code, err := h.readManaged(path, 10000)
			if err != nil || string(code) != tc.content {
				t.Fatalf("source changed: %s %v", code, err)
			}
		})
	}
}

func TestDeployApplicationBlocksMissingAndUnsafeRequirements(t *testing.T) {
	for _, tc := range []struct{ name, id, dir, command string }{
		{"missing code", "site-1", "/home/acme/app", ""},
		{"foreign directory", "site-1", "/home/other/app", "/usr/bin/node server.js"},
		{"unit injection", "site\nUser=root", "/home/acme/app", "/usr/bin/node server.js"},
		{"command injection", "site-1", "/home/acme/app", "/usr/bin/node\nUser=root"},
		{"specifier", "site-1", "/home/acme/%h", "/usr/bin/node server.js"},
		{"relative executable", "site-1", "/home/acme/app", "npm start"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Host{Root: t.TempDir()}
			if _, err := h.deployApplication(tc.id, "acme", "node", tc.dir, tc.command); err == nil {
				t.Fatal("expected requirements error")
			}
			if _, err := os.Stat(h.Root + "/etc/systemd/system/panel-app-site-1.service"); !os.IsNotExist(err) {
				t.Fatal("unit written before validation")
			}
		})
	}
}
