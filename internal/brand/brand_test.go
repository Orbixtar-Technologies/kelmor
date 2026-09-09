package brand

import "testing"

func TestBinaryAlias(t *testing.T) {
	if got := BinaryAlias("panel-api"); got != "kelmor-api" {
		t.Fatalf("api %q", got)
	}
	if got := BinaryAlias("panel-agent"); got != "kelmor-agent" {
		t.Fatalf("agent %q", got)
	}
	if got := BinaryAlias("unknown"); got != "unknown" {
		t.Fatalf("passthrough %q", got)
	}
}

func TestControlPlaneService(t *testing.T) {
	if !ControlPlaneService("panel-api") || !ControlPlaneService("kelmor-worker") {
		t.Fatal("zone A names")
	}
	if ControlPlaneService("panel-agent") || ControlPlaneService("kelmor-agent") {
		t.Fatal("agent is zone B")
	}
}
