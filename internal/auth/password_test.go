package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("s3cret-value")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "s3cret-value") {
		t.Fatal("verify failed")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("false positive")
	}
}
