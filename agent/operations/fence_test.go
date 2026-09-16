package operations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDispatchRejectsStaleFenceAndReplaysReceipt(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	params, err := json.Marshal(map[string]any{
		"username": "fence42", "uid": 20120, "gid": 20120,
		"home": "/home/fence42", "shell": "/usr/sbin/nologin",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := h.Dispatch(context.Background(), Request{
		Method: "CreateLinuxUser",
		Env: Envelope{
			OperationID: "op-1", ResourceID: "acc-fence", Fence: 2,
			ExpectedRevision: 1,
		},
		Params: params,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.Dispatch(context.Background(), Request{
		Method: "CreateLinuxUser",
		Env:    Envelope{OperationID: "op-stale", ResourceID: "acc-fence", Fence: 1},
		Params: params,
	})
	if !errors.Is(err, ErrStaleFence) {
		t.Fatalf("stale fence: %v", err)
	}

	replay, err := h.Dispatch(context.Background(), Request{
		Method: "CreateLinuxUser",
		Env:    Envelope{OperationID: "op-1", ResourceID: "acc-fence", Fence: 2},
		Params: params,
	})
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(v any) map[string]any {
		raw, _ := json.Marshal(v)
		out := map[string]any{}
		_ = json.Unmarshal(raw, &out)
		return out
	}
	firstMap, replayMap := normalize(first), normalize(replay)
	if firstMap["ok"] != replayMap["ok"] || firstMap["observed_state"] != replayMap["observed_state"] || firstMap["message"] != replayMap["message"] {
		t.Fatalf("replay mismatch %#v vs %#v", firstMap, replayMap)
	}
}

func TestDispatchSameFenceDifferentACMETokensWriteBoth(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	env := Envelope{OperationID: "op-cert", ResourceID: "cert-1", Fence: 4}
	firstParams, err := json.Marshal(map[string]any{"token": "tok-a", "body": "body-a"})
	if err != nil {
		t.Fatal(err)
	}
	secondParams, err := json.Marshal(map[string]any{"token": "tok-b", "body": "body-b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Dispatch(context.Background(), Request{Method: "ApplyACMEChallenge", Env: env, Params: firstParams}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Dispatch(context.Background(), Request{Method: "ApplyACMEChallenge", Env: env, Params: secondParams}); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"tok-a", "tok-b"} {
		b, err := os.ReadFile(filepath.Join(h.Root, "var/www/panel-acme/.well-known/acme-challenge", token))
		if err != nil {
			t.Fatalf("%s: %v", token, err)
		}
		if string(b) != "body-"+token[len("tok-"):] {
			t.Fatalf("%s: %q", token, b)
		}
	}
}
