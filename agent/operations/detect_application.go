package operations

import (
	"encoding/json"
	"strings"
)

// DetectResult holds auto-detected project metadata for a working directory.
type DetectResult struct {
	Runtime    string   `json:"runtime"`               // node | python | go | ruby | unknown
	Kind       string   `json:"kind,omitempty"`        // server | spa
	Manager    string   `json:"manager,omitempty"`     // npm | pnpm | yarn (node only)
	InstallCmd string   `json:"install_cmd,omitempty"` // e.g. "npm ci"
	BuildCmd   string   `json:"build_cmd,omitempty"`   // e.g. "pnpm run build"
	StartCmd   string   `json:"start_cmd,omitempty"`   // e.g. "npm start"
	Confidence string   `json:"confidence"`            // detected | inferred | none
	Reasons    []string `json:"reasons,omitempty"`
}

const panelStaticServer = "/var/lib/panel/static-socket-server.cjs"

func nodeManagerBin(manager string) string {
	switch manager {
	case "pnpm":
		return "/usr/local/bin/pnpm"
	case "yarn":
		return "/usr/local/bin/yarn"
	default:
		return "/usr/bin/npm"
	}
}

// DetectApplication inspects the working directory and returns detected runtime
// metadata. It reads files through the managed filesystem so symlinks cannot
// escape the account. It never executes code and is safe on untrusted input.
func (h *Host) DetectApplication(workDir string) (DetectResult, error) {
	read := func(name string) ([]byte, error) {
		return h.readManaged(workDir+"/"+name, 512*1024)
	}
	exists := func(name string) bool {
		_, err := read(name)
		return err == nil
	}

	var result DetectResult
	result.Confidence = "none"

	// --- Node.js ---
	if pkg, err := read("package.json"); err == nil {
		result.Runtime = "node"
		result.Kind = "server"
		result.Confidence = "detected"
		result.Reasons = append(result.Reasons, "found package.json")

		switch {
		case exists("pnpm-lock.yaml"):
			result.Manager = "pnpm"
			result.InstallCmd = "pnpm install --frozen-lockfile"
		case exists("yarn.lock"):
			result.Manager = "yarn"
			result.InstallCmd = "yarn install --frozen-lockfile"
		default:
			result.Manager = "npm"
			result.InstallCmd = "npm ci"
		}
		mgr := nodeManagerBin(result.Manager)

		var manifest struct {
			Scripts map[string]string `json:"scripts"`
			Main    string            `json:"main"`
		}
		if json.Unmarshal(pkg, &manifest) == nil {
			if s := strings.TrimSpace(manifest.Scripts["build"]); s != "" {
				result.BuildCmd = mgr + " run build"
				result.Reasons = append(result.Reasons, "package.json scripts.build present")
			}
			if s := strings.TrimSpace(manifest.Scripts["start"]); s != "" {
				result.StartCmd = mgr + " start"
				result.Reasons = append(result.Reasons, "package.json scripts.start present")
			} else if s := strings.TrimSpace(manifest.Scripts["run"]); s != "" {
				result.StartCmd = mgr + " run run"
			} else if manifest.Main != "" && strings.HasSuffix(manifest.Main, ".js") {
				result.StartCmd = "/usr/bin/node " + manifest.Main
				result.Reasons = append(result.Reasons, "package.json main field: "+manifest.Main)
			}
		}
		if result.StartCmd == "" && exists("server.js") {
			result.StartCmd = "/usr/bin/node server.js"
			result.Reasons = append(result.Reasons, "found server.js")
		}
		if result.StartCmd == "" && exists("index.js") {
			result.StartCmd = "/usr/bin/node index.js"
			result.Reasons = append(result.Reasons, "found index.js")
		}
		if result.StartCmd == "" && result.BuildCmd != "" {
			result.Kind = "spa"
			result.StartCmd = "/usr/bin/node " + panelStaticServer
			result.Reasons = append(result.Reasons, "static SPA: build script without start/server entry")
		}
		return result, nil
	}

	// --- Python ---
	if exists("requirements.txt") || exists("Pipfile") || exists("pyproject.toml") {
		result.Runtime = "python"
		result.Kind = "server"
		result.Confidence = "detected"
		switch {
		case exists("requirements.txt"):
			result.InstallCmd = "pip install -r requirements.txt"
			result.Reasons = append(result.Reasons, "found requirements.txt")
		case exists("Pipfile"):
			result.InstallCmd = "pipenv install"
			result.Reasons = append(result.Reasons, "found Pipfile")
		case exists("pyproject.toml"):
			result.InstallCmd = "pip install ."
			result.Reasons = append(result.Reasons, "found pyproject.toml")
		}
		for _, name := range []string{"app.py", "main.py", "wsgi.py", "asgi.py", "server.py", "run.py"} {
			if exists(name) {
				result.StartCmd = "/usr/bin/python3 " + name
				result.Reasons = append(result.Reasons, "found "+name)
				break
			}
		}
		return result, nil
	}

	// --- Go ---
	if exists("go.mod") {
		result.Runtime = "go"
		result.Kind = "server"
		result.Confidence = "detected"
		result.InstallCmd = "go build -o app ."
		result.StartCmd = "./app"
		result.Reasons = append(result.Reasons, "found go.mod")
		return result, nil
	}

	// --- Ruby ---
	if exists("Gemfile") {
		result.Runtime = "ruby"
		result.Kind = "server"
		result.Confidence = "detected"
		result.InstallCmd = "bundle install"
		result.Reasons = append(result.Reasons, "found Gemfile")
		if exists("config.ru") {
			result.StartCmd = "bundle exec rackup config.ru"
			result.Reasons = append(result.Reasons, "found config.ru")
		} else if exists("app.rb") {
			result.StartCmd = "bundle exec ruby app.rb"
			result.Reasons = append(result.Reasons, "found app.rb")
		}
		return result, nil
	}

	if exists("server.js") || exists("index.js") || exists("app.js") {
		result.Runtime = "node"
		result.Kind = "server"
		result.Confidence = "inferred"
		if exists("server.js") {
			result.StartCmd = "/usr/bin/node server.js"
		} else if exists("index.js") {
			result.StartCmd = "/usr/bin/node index.js"
		} else {
			result.StartCmd = "/usr/bin/node app.js"
		}
		result.Reasons = append(result.Reasons, "found JS entrypoint without package.json")
		return result, nil
	}
	if exists("app.py") || exists("main.py") {
		result.Runtime = "python"
		result.Kind = "server"
		result.Confidence = "inferred"
		if exists("app.py") {
			result.StartCmd = "/usr/bin/python3 app.py"
		} else {
			result.StartCmd = "/usr/bin/python3 main.py"
		}
		result.Reasons = append(result.Reasons, "found Python entrypoint without requirements.txt")
		return result, nil
	}

	result.Runtime = "unknown"
	return result, nil
}

