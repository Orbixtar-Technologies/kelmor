package wordpress

import "testing"

func TestValidateArgs(t *testing.T) {
	if err := ValidateArgs([]string{"core", "download"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateArgs([]string{"rm", "-rf"}); err == nil {
		t.Fatal("expected reject")
	}
	if err := ValidateArgs([]string{"core", "download;id"}); err == nil {
		t.Fatal("expected reject")
	}
}
