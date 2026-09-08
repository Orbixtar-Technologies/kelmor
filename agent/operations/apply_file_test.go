package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyFileChunkAssembles(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	body := []byte("hello-chunked-file")
	if _, err := h.applyFileChunk("/var/lib/panel/backups/staging/a.bin", body[:5], 0o640, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyFileChunk("/var/lib/panel/backups/staging/a.bin", body[5:], 0o640, 5, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "var/lib/panel/backups/staging/a.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q", got)
	}
}

func TestApplyFileOverUnixChunks(t *testing.T) {
	root := t.TempDir()
	sock := filepath.Join(root, "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host := &Host{Root: root}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 64*1024), MaxRPCLine)
			for sc.Scan() {
				var req Request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := host.Dispatch(context.Background(), req)
				out := map[string]any{"ok": err == nil, "result": res}
				if err != nil {
					out["error"] = err.Error()
				}
				b, _ := json.Marshal(out)
				_, _ = c.Write(append(b, '\n'))
			}
			_ = c.Close()
		}
	}()
	client := &Host{Sock: sock}
	want := make([]byte, fileChunkSize+80)
	for i := range want {
		want[i] = byte(i)
	}
	if _, err := client.ApplyFile("/var/lib/panel/backups/staging/big.bin", want, 0o640); err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	<-done
	got, err := os.ReadFile(filepath.Join(root, "var/lib/panel/backups/staging/big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("len %d != %d", len(got), len(want))
	}
}
