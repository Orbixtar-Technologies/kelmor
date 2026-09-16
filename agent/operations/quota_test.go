package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSFTPReadOnly(t *testing.T) {
	if SFTPReadOnly(7, 0) {
		t.Fatal("unlimited")
	}
	if SFTPReadOnly(7, 10) {
		t.Fatal("under")
	}
	if !SFTPReadOnly(10, 10) {
		t.Fatal("at cap")
	}
}

func TestSetQuotaPersistsAndApplyFileRejects(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	home := filepath.Join(root, "home", "acme42", "public_html")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := h.setQuota("acme42", 8); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "var/lib/panel/quotas/acme42")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "home/acme42/.panel-quota")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ApplyFile("/home/acme42/public_html/big.txt", []byte("0123456789"), 0o644); err == nil {
		t.Fatal("expected disk limit")
	}
	if _, err := h.ApplyFile("/home/acme42/public_html/ok.txt", []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnforceAccountDiskLocksHomeAndSSHD(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	pub := filepath.Join(root, "home", "lock42", "public_html")
	if err := os.MkdirAll(pub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pub, "fat.bin"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.setQuota("lock42", 8); err != nil {
		t.Fatal(err)
	}
	res, err := h.enforceAccountDisk("lock42", "/home/lock42")
	if err != nil {
		t.Fatal(err)
	}
	if res.ObservedState != "readonly" {
		t.Fatalf("%+v", res)
	}
	st, err := os.Stat(pub)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o550 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
	conf, err := os.ReadFile(filepath.Join(root, "etc/ssh/sshd_config.d/zz-panel-sftp-quota.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conf), "Match User lock42") || !strings.Contains(string(conf), "internal-sftp -R") {
		t.Fatalf("%s", conf)
	}
	if err := h.setHomeWriteLock("lock42", false); err != nil {
		t.Fatal(err)
	}
}

func TestChunkedReplacementQuotaUsesProjectedFinalSize(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	home := filepath.Join(root, "home", "acme42", "public_html")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	existing := bytesOf('a', 100)
	if err := os.WriteFile(filepath.Join(home, "index.html"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.persistQuota("acme42", 250); err != nil {
		t.Fatal(err)
	}
	first := bytesOf('b', 100)
	if _, err := h.applyFileChunk("/home/acme42/public_html/index.html", first, 0o640, 0, false); err != nil {
		t.Fatal(err)
	}
	second := bytesOf('c', 100)
	if _, err := h.applyFileChunk("/home/acme42/public_html/index.html", second, 0o640, 100, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(home, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 200 {
		t.Fatalf("len %d", len(got))
	}
}

func TestChunkedReplacementQuotaRejectsWhenProjectedExceeds(t *testing.T) {
	root := t.TempDir()
	h := &Host{Root: root}
	home := filepath.Join(root, "home", "acme42", "public_html")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "index.html"), bytesOf('a', 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.persistQuota("acme42", 150); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyFileChunk("/home/acme42/public_html/index.html", bytesOf('b', 100), 0o640, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := h.applyFileChunk("/home/acme42/public_html/index.html", bytesOf('c', 100), 0o640, 100, true); err == nil {
		t.Fatal("projected 200-byte file must exceed 150-byte quota")
	}
}

func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
