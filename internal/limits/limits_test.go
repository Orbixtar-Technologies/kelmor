package limits

import "testing"

func TestEnforceUnlimitedWhenLimitZero(t *testing.T) {
	if err := Enforce(99, 0, "domains"); err != nil {
		t.Fatal(err)
	}
}

func TestEnforceBlocksAtCap(t *testing.T) {
	err := Enforce(1, 1, "domains")
	if err == nil {
		t.Fatal("expected limit")
	}
	c, ok := err.(Check)
	if !ok || c.Kind != "domains" || c.Used != 1 || c.Limit != 1 {
		t.Fatalf("%#v", err)
	}
}

func TestDiskWouldExceed(t *testing.T) {
	if err := DiskWouldExceed(8, 1, 10); err != nil {
		t.Fatal(err)
	}
	if err := DiskWouldExceed(8, 3, 10); err == nil {
		t.Fatal("expected disk limit")
	}
}
