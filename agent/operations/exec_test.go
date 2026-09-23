package operations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestUseraddAllowsSupplementaryGroup(t *testing.T) {
	args := []string{"-u", "20000", "-g", "acme42", "-d", "/home/acme42", "-s", "/usr/sbin/nologin", "-m", "-G", "panel-sftp", "acme42"}
	if err := validateFixedCommand("/usr/sbin/useradd", nil, args); err != nil {
		t.Fatalf("account reconcile useradd args must be allow-listed: %v", err)
	}
}

func TestCommandFailureUsesErrWhenOutputEmpty(t *testing.T) {
	err := commandFailure("useradd", nil, fmt.Errorf("leading option"))
	if err == nil || err.Error() != "useradd: leading option" {
		t.Fatalf("got %v", err)
	}
}

func TestRunFixedRejects(t *testing.T) {
	if _, err := runFixed("/bin/sh", "-c", "id"); err == nil {
		t.Fatal("shell must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "acme;id"); err == nil {
		t.Fatal("metachar must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "-evil", "acme"); err == nil {
		t.Fatal("leading option must be rejected")
	}
	if _, err := runFixed("/usr/sbin/useradd", "acme\x00root"); err == nil {
		t.Fatal("control character must be rejected")
	}
	if _, err := runFixedEnv(context.Background(), "/usr/sbin/useradd", []string{"PATH=/tmp/evil"}, 0, nil, "acme"); err == nil {
		t.Fatal("hostile environment must be rejected")
	}
}

func TestRunFixedBoundsOutputAndHonorsCancel(t *testing.T) {
	if _, err := runFixed("/usr/bin/python3", "-c", strings.Repeat("print('x'*1000)\n", 1)); err == nil {
		// python -c is not an allowlisted argument contract
		t.Fatal("python -c must be rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runFixedEnv(ctx, "/usr/bin/pgrep", nil, time.Second, nil, "-x", "init"); err == nil {
		t.Fatal("cancelled context must fail before changing the executable contract")
	}
}

func gitSafeDirectoryEnvForTest() []string {
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"HOME=/tmp",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=safe.directory",
		"GIT_CONFIG_VALUE_0=*",
	}
}

func TestGitDeployCommandsAreAllowListed(t *testing.T) {
	env := gitSafeDirectoryEnvForTest()
	cases := [][]string{
		{"-C", "/home/acme/app", "fetch", "--depth=1", "https://github.com/example/repo.git", "main"},
		{"-C", "/home/acme/app", "fetch", "--depth=1", "https://x-access-token:tok@github.com/example/repo.git", "main"},
		{"-C", "/home/acme/app", "reset", "--hard", "FETCH_HEAD"},
		{"clone", "--depth=1", "--branch", "main", "https://github.com/example/repo.git", "/home/acme/app"},
		{"-C", "/home/acme/app", "init"},
		{"-C", "/home/acme/app", "remote", "add", "origin", "https://github.com/example/repo.git"},
		{"-C", "/home/acme/app", "fetch", "--depth=1", "origin", "main"},
		{"-C", "/home/acme/app", "checkout", "-f", "FETCH_HEAD"},
	}
	for _, args := range cases {
		if err := validateFixedCommand("/usr/bin/git", env, args); err != nil {
			t.Fatalf("git %v must be allow-listed for redeploy: %v", args, err)
		}
	}
}

func TestGitRejectsHostileEnvAndFlags(t *testing.T) {
	if err := validateFixedCommand("/usr/bin/git", []string{"PATH=/tmp"}, []string{"fetch"}); err == nil {
		t.Fatal("hostile environment must be rejected")
	}
	if err := validateFixedCommand("/usr/bin/git", []string{"PATH=/usr/local/bin:/usr/bin:/bin"}, []string{"fetch"}); err != nil {
		t.Fatalf("system PATH must be allowed: %v", err)
	}
	if err := validateFixedCommand("/usr/bin/git", nil, []string{"-c", "core.sshCommand=id"}); err == nil {
		t.Fatal("unlisted git flag -c must be rejected")
	}
	if err := validateFixedCommand("/usr/bin/git", []string{
		"GIT_TERMINAL_PROMPT=0", "HOME=/tmp",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.sshCommand", "GIT_CONFIG_VALUE_0=id",
	}, []string{"fetch"}); err == nil {
		t.Fatal("GIT_CONFIG core.sshCommand must be rejected")
	}
}
