package operations

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hosting-panel/panel/agent/policy"
)

// gitSafeDirectoryEnv marks every directory as safe for git when the agent runs
// as root against account-owned working trees (CVE-2022-24765 ownership check).
// Prefer GIT_CONFIG_* over `git -c`, which is intentionally not allow-listed.
func gitSafeDirectoryEnv() []string {
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"HOME=/tmp",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=safe.directory",
		"GIT_CONFIG_VALUE_0=*",
	}
}

// gitDeployApplication clones or pulls a git repository into workDir, auto-detects
// the runtime if needed, and deploys the application. When git_auth is set for an
// HTTPS URL it is embedded in the clone/fetch URL (same as a one-shot credential)
// and redacted from surfaced errors.
func (h *Host) gitDeployApplication(websiteID, account, runtime, workDir, gitURL, gitBranch, gitAuth, startCmdOverride string) (Result, error) {
	if err := validateGitURL(gitURL); err != nil {
		return Result{}, fmt.Errorf("git: %w", err)
	}
	if gitBranch == "" {
		gitBranch = "main"
	}
	canonical, err := policy.WithinAccount(account, workDir)
	if err != nil || canonical != workDir {
		return Result{}, fmt.Errorf("working directory must be canonical and within the account")
	}

	if err := h.syncGitWorkDir(workDir, gitURL, gitBranch, gitAuth); err != nil {
		return Result{}, err
	}
	h.chownTree(account, workDir)

	detected, _ := h.DetectApplication(workDir)

	resolvedRuntime := runtime
	if resolvedRuntime == "" {
		switch detected.Runtime {
		case "node", "python":
			resolvedRuntime = detected.Runtime
		default:
			resolvedRuntime = "node"
		}
	}

	startCmd := startCmdOverride
	if startCmd == "" {
		startCmd = detected.StartCmd
	}

	return h.deployApplication(websiteID, account, resolvedRuntime, workDir, startCmd)
}

// syncGitWorkDir clones or updates workDir. A non-empty destination without
// .git is reused via init/fetch/checkout so delete leftovers (public_html
// placeholders) cannot fail recreate with "already exists and is not empty".
func (h *Host) syncGitWorkDir(workDir, gitURL, gitBranch, gitAuth string) error {
	realDir, err := h.resolve(workDir)
	if err != nil {
		return fmt.Errorf("git: working directory: %w", err)
	}
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		return fmt.Errorf("git: could not create working directory: %w", err)
	}

	cloneURL := buildAuthURL(gitURL, gitAuth)
	gitEnv := gitSafeDirectoryEnv()
	gitDir := filepath.Join(realDir, ".git")
	if info, statErr := os.Stat(gitDir); statErr == nil && info.IsDir() {
		out, err := runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 120*time.Second, nil,
			"-C", realDir, "fetch", "--depth=1", cloneURL, gitBranch)
		if err != nil {
			return gitOpError("git fetch", out, err, gitAuth)
		}
		out, err = runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 60*time.Second, nil,
			"-C", realDir, "reset", "--hard", "FETCH_HEAD")
		if err != nil {
			return gitOpError("git reset", out, err, gitAuth)
		}
		return nil
	}

	empty, err := dirEmpty(realDir)
	if err != nil {
		return fmt.Errorf("git: inspect working directory: %w", err)
	}
	if empty {
		out, err := runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 180*time.Second, nil,
			"clone", "--depth=1", "--branch", gitBranch, cloneURL, realDir)
		if err != nil {
			return gitOpError("git clone", out, err, gitAuth)
		}
		return nil
	}

	// Non-empty leftover (panel index.html, previous failed clone, etc.).
	out, err := runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 60*time.Second, nil,
		"-C", realDir, "init")
	if err != nil {
		return gitOpError("git init", out, err, gitAuth)
	}
	_, _ = runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 15*time.Second, nil,
		"-C", realDir, "remote", "remove", "origin")
	out, err = runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 30*time.Second, nil,
		"-C", realDir, "remote", "add", "origin", cloneURL)
	if err != nil {
		return gitOpError("git remote add", out, err, gitAuth)
	}
	out, err = runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 180*time.Second, nil,
		"-C", realDir, "fetch", "--depth=1", "origin", gitBranch)
	if err != nil {
		return gitOpError("git fetch", out, err, gitAuth)
	}
	out, err = runFixedEnv(h.commandContext(), "/usr/bin/git", gitEnv, 60*time.Second, nil,
		"-C", realDir, "checkout", "-f", "FETCH_HEAD")
	if err != nil {
		return gitOpError("git checkout", out, err, gitAuth)
	}
	return nil
}

func dirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func gitOpError(op string, out []byte, err error, secrets ...string) error {
	msg := strings.TrimSpace(string(out))
	if msg == "" && err != nil {
		msg = err.Error()
	}
	msg = redactGitSecrets(msg, secrets...)
	if msg == "" {
		if err != nil {
			return fmt.Errorf("%s: %v", op, err)
		}
		return fmt.Errorf("%s failed", op)
	}
	if len(msg) > 512 {
		msg = msg[:512] + "…"
	}
	return fmt.Errorf("%s: %s", op, msg)
}

func redactGitSecrets(msg string, secrets ...string) string {
	for _, s := range secrets {
		if s == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, s, "[redacted]")
	}
	for {
		start := strings.Index(msg, "https://")
		if start < 0 {
			break
		}
		rest := msg[start+len("https://"):]
		at := strings.IndexByte(rest, '@')
		slash := strings.IndexByte(rest, '/')
		if at < 0 || (slash >= 0 && at > slash) {
			break
		}
		msg = msg[:start] + "https://[redacted]@" + rest[at+1:]
	}
	return msg
}

func validateGitURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("git_url is required")
	}
	if strings.HasPrefix(rawURL, "git@") {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid git URL")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		return fmt.Errorf("http git URLs are not allowed; use https or SSH")
	default:
		return fmt.Errorf("unsupported git URL scheme %q; use https or git@", u.Scheme)
	}
}

func buildAuthURL(rawURL, token string) string {
	if token == "" || strings.HasPrefix(rawURL, "git@") {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return rawURL
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String()
}
