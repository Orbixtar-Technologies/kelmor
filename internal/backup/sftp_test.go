package backup

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestRemoteSFTPPutGet(t *testing.T) {
	root := t.TempDir()
	addr, fp := startTestSFTP(t, "panel-backup", "OffsitePass!2026")
	s := &SFTP{
		Root:     root,
		Host:     addr,
		User:     "panel-backup",
		Password: "OffsitePass!2026",
		HostKey:  fp,
	}
	ctx := context.Background()
	if err := s.Put(ctx, "acct/one.hpm", []byte("HPM1-offsite")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "acct/one.hpm")
	if err != nil || string(got) != "HPM1-offsite" {
		t.Fatalf("%q %v", got, err)
	}
	st, err := s.Stat(ctx, "acct/one.hpm")
	if err != nil || st.Size != 12 {
		t.Fatalf("%+v %v", st, err)
	}
	items, err := s.List(ctx, "acct")
	if err != nil || len(items) == 0 {
		t.Fatalf("list %v %v", items, err)
	}
	if _, err := os.Stat(filepath.Join(root, "acct/one.hpm")); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteSFTPRejectsBadHostKey(t *testing.T) {
	root := t.TempDir()
	addr, _ := startTestSFTP(t, "panel-backup", "OffsitePass!2026")
	s := &SFTP{
		Root: root, Host: addr, User: "panel-backup",
		Password: "OffsitePass!2026", HostKey: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}
	if err := s.Put(context.Background(), "x.hpm", []byte("no")); err == nil {
		t.Fatal("must reject host key")
	}
}

func TestSFTPFromEnvLocalFallback(t *testing.T) {
	t.Setenv("PANEL_SFTP_HOST", "")
	t.Setenv("PANEL_SFTP_ROOT", t.TempDir())
	s := SFTPFromEnv()
	if s.remote() {
		t.Fatal("empty host is local")
	}
	if err := s.Put(context.Background(), "a.hpm", []byte("x")); err != nil {
		t.Fatal(err)
	}
}

func startTestSFTP(t *testing.T, user, pass string) (addr, fingerprint string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, secret []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(secret) == pass {
				return nil, nil
			}
			return nil, os.ErrPermission
		},
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			nConn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSFTPConn(nConn, cfg)
		}
	}()
	return ln.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

func serveSFTPConn(nConn net.Conn, cfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		_ = nConn.Close()
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "")
			continue
		}
		ch, incoming, err := newCh.Accept()
		if err != nil {
			continue
		}
		go func(ch ssh.Channel, incoming <-chan *ssh.Request) {
			defer ch.Close()
			for req := range incoming {
				if req.Type == "subsystem" && subsystemName(req.Payload) == "sftp" {
					_ = req.Reply(true, nil)
					srv, err := sftp.NewServer(ch)
					if err != nil {
						return
					}
					_ = srv.Serve()
					return
				}
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
			}
		}(ch, incoming)
	}
}

func subsystemName(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	n := int(binary.BigEndian.Uint32(payload[:4]))
	if n < 0 || 4+n > len(payload) {
		return ""
	}
	return string(payload[4 : 4+n])
}
