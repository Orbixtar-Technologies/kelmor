package operations

import (
	"strings"
	"testing"
)

func TestRenderSystemCronIncludesUser(t *testing.T) {
	body, err := renderSystemCron("acme42", "# panel crontab\n0 * * * * php cron.php\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "0 * * * * acme42 php cron.php") {
		t.Fatalf("%q", body)
	}
}

func TestRenderSystemCronRejectsMetacharacters(t *testing.T) {
	if _, err := renderSystemCron("acme42", "* * * * * rm -rf /; echo pwned"); err == nil {
		t.Fatal("expected reject")
	}
}
