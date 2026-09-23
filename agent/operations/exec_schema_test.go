package operations

import "testing"


func TestValidateRunuserAllowsNestedPnpmFlags(t *testing.T) {
	args := []string{
		"-u", "rswaters", "--",
		"/usr/bin/env",
		"HOME=/home/rswaters",
		"USER=rswaters",
		"LOGNAME=rswaters",
		"CI=1",
		"NODE_OPTIONS=--max-old-space-size=384",
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"PNPM_STORE_DIR=/home/rswaters/apps/.pnpm-store",
		"COREPACK_HOME=/home/rswaters/.cache/node/corepack",
		"/usr/local/bin/pnpm",
		"--dir", "/home/rswaters/public_html",
		"install", "--frozen-lockfile", "--force",
	}
	if err := validateFixedCommand("/usr/sbin/runuser", nil, args); err != nil {
		t.Fatalf("expected nested pnpm flags to pass: %v", err)
	}
}

func TestValidateRunuserRejectsUnknownNestedFlag(t *testing.T) {
	args := []string{"-u", "rswaters", "--", "/usr/local/bin/pnpm", "--unknown-flag", "x"}
	if err := validateFixedCommand("/usr/sbin/runuser", nil, args); err == nil {
		t.Fatal("expected unknown nested flag to fail")
	}
}
