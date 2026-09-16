package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hosting-panel/panel/internal/pkg/secret"
	"github.com/hosting-panel/panel/internal/store"
)

func testBox(t *testing.T, fill byte) *secret.Box {
	t.Helper()
	box, err := secret.FromBytes(bytes.Repeat([]byte{fill}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func testAccount() *store.Account {
	return &store.Account{ID: "acct-1", Username: "acme42", HomePath: "/home/acme42"}
}

func TestHPM3RoundTripVerifies(t *testing.T) {
	box := testBox(t, 9)
	repo := &Local{Root: t.TempDir()}
	home, err := PackHome(writeHome(t, "hello-hpm3"))
	if err != nil {
		t.Fatal(err)
	}
	parts := []Component{
		{Name: ComponentHome, Data: home},
		{Name: "databases/mariadb/site", Data: []byte("SELECT 1;\n")},
		{Name: "mail/acme.test/info", Data: []byte("maildir")},
	}
	anchor := ConsistencyAnchor{Fence: 4, Level: "account", Quiesced: []string{"files", "databases", "mail"}, SnapshotAt: "2026-09-16T00:00:00Z"}
	man, key, err := SealHPM3(context.Background(), box, repo, testAccount(), parts, expectedFrom(parts), anchor)
	if err != nil {
		t.Fatal(err)
	}
	if man.FormatVersion != FormatHPM3 {
		t.Fatalf("format %d", man.FormatVersion)
	}
	if man.KeyIdentity == "" || man.Checksums["object"] == "" {
		t.Fatalf("missing identities %+v", man)
	}
	if man.Consistency.Fence != 4 {
		t.Fatalf("anchor %+v", man.Consistency)
	}
	opened, err := OpenAny(context.Background(), box, repo, key)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if opened.Manifest.FormatVersion != FormatHPM3 {
		t.Fatalf("opened format %d", opened.Manifest.FormatVersion)
	}
	got := map[string][]byte{}
	if err := opened.Each(func(name string, r io.Reader) error {
		body, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		got[name] = body
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if string(got["databases/mariadb/site"]) != "SELECT 1;\n" || string(got["mail/acme.test/info"]) != "maildir" {
		t.Fatalf("%v", got)
	}
	dest := t.TempDir()
	if err := UnpackHomeBytes(got[ComponentHome], dest); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dest, "public_html", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello-hpm3" {
		t.Fatalf("got %q", body)
	}
}

func TestHPM3RejectsCorruptComponents(t *testing.T) {
	box := testBox(t, 9)
	repo := &Local{Root: t.TempDir()}
	parts := []Component{
		{Name: ComponentHome, Data: []byte("home-bytes")},
		{Name: "databases/mariadb/site", Data: []byte("SQL")},
	}
	expected := expectedFrom(parts)
	anchor := ConsistencyAnchor{Fence: 1, Level: "account", SnapshotAt: "t"}
	cases := []struct {
		name  string
		parts []Component
		want  string
	}{
		{"missing", []Component{parts[0]}, "missing"},
		{"duplicate", []Component{parts[0], parts[1], parts[1]}, "duplicate"},
		{"reordered", []Component{parts[1], parts[0]}, "order"},
		{"empty-home", []Component{{Name: ComponentHome, Data: nil}, parts[1]}, "empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := SealHPM3(context.Background(), box, repo, testAccount(), tc.parts, expected, anchor)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("got %v", err)
			}
		})
	}

	man, key, err := SealHPM3(context.Background(), box, repo, testAccount(), parts, expected, anchor)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := repo.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	obj[len(obj)-1] ^= 0xff
	if err := repo.Put(context.Background(), key, obj); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAny(context.Background(), box, repo, key); err == nil {
		t.Fatal("corrupted object opened")
	}
	_ = man
}

func TestHPM3RejectsTruncatedAndUnreadable(t *testing.T) {
	box := testBox(t, 9)
	repo := &Local{Root: t.TempDir()}
	parts := []Component{{Name: ComponentHome, Data: bytes.Repeat([]byte("h"), 1024)}}
	_, key, err := SealHPM3(context.Background(), box, repo, testAccount(), parts, expectedFrom(parts), ConsistencyAnchor{Fence: 1, Level: "account"})
	if err != nil {
		t.Fatal(err)
	}
	obj, err := repo.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Put(context.Background(), key, obj[:len(obj)/2]); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenAny(context.Background(), box, repo, key)
	if err == nil {
		err = opened.Each(func(string, io.Reader) error { return nil })
		_ = opened.Close()
	}
	if err == nil {
		t.Fatal("truncated object accepted")
	}
	_, _, err = SealHPM3(context.Background(), box, repo, testAccount(), []Component{{
		Name: ComponentHome,
		Open: func() (io.ReadCloser, int64, error) {
			return nil, 0, errors.New("mailbox unreadable")
		},
	}}, []string{ComponentHome}, ConsistencyAnchor{Fence: 1, Level: "account"})
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("got %v", err)
	}
}

func TestKeyRotationRestoresRetainedGenerations(t *testing.T) {
	oldBox := testBox(t, 3)
	newBox := testBox(t, 4)
	repo := &Local{Root: t.TempDir()}
	parts := []Component{{Name: ComponentHome, Data: []byte("gen-old")}}
	_, oldKey, err := SealHPM3ToKey(context.Background(), oldBox, repo, testAccount(), "acme42/gen-old.hpm", parts, expectedFrom(parts), ConsistencyAnchor{Fence: 1, Level: "account"})
	if err != nil {
		t.Fatal(err)
	}
	ring, err := KeyRingFromBoxes(newBox, oldBox)
	if err != nil {
		t.Fatal(err)
	}
	_, newKey, err := SealHPM3WithRingToKey(context.Background(), ring, repo, testAccount(), "acme42/gen-new.hpm", []Component{{Name: ComponentHome, Data: []byte("gen-new")}}, []string{ComponentHome}, ConsistencyAnchor{Fence: 2, Level: "account"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{oldKey, newKey} {
		opened, err := OpenAnyWithRing(context.Background(), ring, repo, key)
		if err != nil {
			t.Fatal(err)
		}
		if err := opened.Each(func(string, io.Reader) error { return nil }); err != nil {
			t.Fatal(err)
		}
		_ = opened.Close()
	}
	if _, err := OpenAny(context.Background(), newBox, repo, oldKey); err == nil {
		t.Fatal("new box alone must not open old generation")
	}
	revoked, err := KeyRingFromBox(newBox)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAnyWithRing(context.Background(), revoked, repo, oldKey); err == nil {
		t.Fatal("revoked old identity opened backup")
	}
}

func TestWrongAndTamperedKeyIdentityFail(t *testing.T) {
	box := testBox(t, 9)
	other := testBox(t, 8)
	repo := &Local{Root: t.TempDir()}
	parts := []Component{{Name: ComponentHome, Data: []byte("secret-home")}}
	_, key, err := SealHPM3(context.Background(), box, repo, testAccount(), parts, expectedFrom(parts), ConsistencyAnchor{Fence: 1, Level: "account"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAny(context.Background(), other, repo, key); err == nil {
		t.Fatal("wrong key opened archive")
	}
}

func TestSealRequiresConsistencyAnchor(t *testing.T) {
	box := testBox(t, 9)
	repo := &Local{Root: t.TempDir()}
	parts := []Component{{Name: ComponentHome, Data: []byte("x")}}
	_, _, err := SealHPM3(context.Background(), box, repo, testAccount(), parts, expectedFrom(parts), ConsistencyAnchor{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "consistency") {
		t.Fatalf("got %v", err)
	}
}

func TestHPM3SealStaysWithinMemoryCeiling(t *testing.T) {
	box := testBox(t, 9)
	const payload = 32 << 20
	const ceiling = 8 << 20
	repo := &rejectLargePut{Local: Local{Root: t.TempDir()}, limit: 1 << 20}
	parts := []Component{{
		Name: ComponentHome,
		Open: func() (io.ReadCloser, int64, error) {
			return io.NopCloser(io.LimitReader(repeatReader{b: 'A'}, payload)), payload, nil
		},
		Size: payload,
	}}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	_, _, err := SealHPM3(context.Background(), box, repo, testAccount(), parts, []string{ComponentHome}, ConsistencyAnchor{Fence: 1, Level: "account"})
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapInuse > before.HeapInuse && after.HeapInuse-before.HeapInuse > ceiling {
		t.Fatalf("heap grew %d (ceiling %d)", after.HeapInuse-before.HeapInuse, ceiling)
	}
}

func TestReconcileAfterUploadWithoutMetadata(t *testing.T) {
	box := testBox(t, 9)
	repo := &Local{Root: t.TempDir()}
	parts := []Component{{Name: ComponentHome, Data: []byte("home")}}
	run := &store.BackupRun{ID: "bak-1", AccountID: "acct-1", State: "queued", Destination: "local"}
	key := ObjectKey(testAccount(), "2026-09-16T00:00:00Z")
	if err := PersistObjectKey(run, key); err != nil {
		t.Fatal(err)
	}
	if run.State != "uploading" || run.Manifest["key"] != key {
		t.Fatalf("%+v", run)
	}
	man, gotKey, err := SealHPM3ToKey(context.Background(), box, repo, testAccount(), key, parts, expectedFrom(parts), ConsistencyAnchor{Fence: 1, Level: "account"})
	if err != nil || gotKey != key {
		t.Fatalf("%s %v", gotKey, err)
	}
	if err := ReconcileUploaded(context.Background(), repo, run, man); err != nil {
		t.Fatal(err)
	}
	if run.State != "succeeded" || run.Checksum == "" {
		t.Fatalf("%+v", run)
	}
}

func writeHome(t *testing.T, body string) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(filepath.Join(home, "public_html"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "public_html", "index.html"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func expectedFrom(parts []Component) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.Name
	}
	return out
}

type repeatReader struct{ b byte }

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b
	}
	return len(p), nil
}

type rejectLargePut struct {
	Local
	limit int
}

func (r *rejectLargePut) Put(ctx context.Context, key string, data []byte) error {
	if len(data) > r.limit {
		return errors.New("Put materialized the payload")
	}
	return r.Local.Put(ctx, key, data)
}
