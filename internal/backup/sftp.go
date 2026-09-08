package backup

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTP writes HPM1 objects to a remote directory over SSH when
// PANEL_SFTP_HOST is set. Without a host it uses PANEL_SFTP_ROOT as a
// local staging tree (mounted chroot or this node's offsite disk).
type SFTP struct {
	Root        string
	Host        string
	User        string
	Password    string
	KeyPEM      []byte
	HostKey     string
	DialTimeout time.Duration
	dial        func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error)
}

func SFTPFromEnv() *SFTP {
	root := os.Getenv("PANEL_SFTP_ROOT")
	if root == "" {
		root = "/var/lib/panel/offsite"
	}
	keyPEM := []byte(os.Getenv("PANEL_SFTP_KEY_PEM"))
	if len(keyPEM) == 0 {
		if p := os.Getenv("PANEL_SFTP_KEY"); p != "" {
			keyPEM, _ = os.ReadFile(p)
		}
	}
	timeout := 20 * time.Second
	return &SFTP{
		Root:        root,
		Host:        os.Getenv("PANEL_SFTP_HOST"),
		User:        envDefault("PANEL_SFTP_USER", "panel-backup"),
		Password:    os.Getenv("PANEL_SFTP_PASSWORD"),
		KeyPEM:      keyPEM,
		HostKey:     os.Getenv("PANEL_SFTP_HOST_KEY"),
		DialTimeout: timeout,
	}
}

func (s *SFTP) remote() bool {
	return s != nil && strings.TrimSpace(s.Host) != ""
}

func (s *SFTP) Put(ctx context.Context, key string, data []byte) error {
	if !s.remote() {
		return writeAtomic(s.Root, key, data)
	}
	return s.withClient(ctx, func(c *sftp.Client) error {
		p := s.RemotePath(key)
		if err := c.MkdirAll(path.Dir(p)); err != nil {
			return err
		}
		f, err := c.OpenFile(p+".staging", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		_ = c.Remove(p)
		return c.Rename(p+".staging", p)
	})
}

func (s *SFTP) Get(ctx context.Context, key string) ([]byte, error) {
	if !s.remote() {
		return readPath(s.Root, key)
	}
	var body []byte
	err := s.withClient(ctx, func(c *sftp.Client) error {
		f, err := c.Open(s.RemotePath(key))
		if err != nil {
			return err
		}
		defer f.Close()
		body, err = io.ReadAll(f)
		return err
	})
	return body, err
}

func (s *SFTP) Stat(ctx context.Context, key string) (Object, error) {
	if !s.remote() {
		b, err := readPath(s.Root, key)
		if err != nil {
			return Object{}, err
		}
		return Object{Key: key, Size: int64(len(b))}, nil
	}
	var sz int64
	err := s.withClient(ctx, func(c *sftp.Client) error {
		st, err := c.Stat(s.RemotePath(key))
		if err != nil {
			return err
		}
		sz = st.Size()
		return nil
	})
	if err != nil {
		return Object{}, err
	}
	return Object{Key: key, Size: sz}, nil
}

func (s *SFTP) Delete(ctx context.Context, key string) error {
	if !s.remote() {
		return removePath(s.Root, key)
	}
	return s.withClient(ctx, func(c *sftp.Client) error {
		return c.Remove(s.RemotePath(key))
	})
}

func (s *SFTP) List(ctx context.Context, prefix string) ([]Object, error) {
	if !s.remote() {
		return listPrefix(s.Root, prefix)
	}
	var out []Object
	err := s.withClient(ctx, func(c *sftp.Client) error {
		ents, err := c.ReadDir(s.RemotePath(prefix))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range ents {
			out = append(out, Object{Key: path.Join(prefix, e.Name()), Size: e.Size()})
		}
		return nil
	})
	return out, err
}

func (s *SFTP) Verify(ctx context.Context, key, checksum string) error {
	b, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if checksum != "" && len(b) == 0 {
		return fmt.Errorf("empty sftp object")
	}
	return nil
}

func (s *SFTP) RemotePath(key string) string {
	return path.Join(s.Root, strings.TrimPrefix(key, "/"))
}

func (s *SFTP) withClient(ctx context.Context, fn func(*sftp.Client) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg, addr, err := s.sshConfig()
	if err != nil {
		return err
	}
	dial := s.dial
	if dial == nil {
		dial = func(network, a string, c *ssh.ClientConfig) (*ssh.Client, error) {
			d := net.Dialer{Timeout: s.timeout()}
			conn, err := d.DialContext(ctx, network, a)
			if err != nil {
				return nil, err
			}
			cc, chans, reqs, err := ssh.NewClientConn(conn, a, c)
			if err != nil {
				_ = conn.Close()
				return nil, err
			}
			return ssh.NewClient(cc, chans, reqs), nil
		}
	}
	cli, err := dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	defer cli.Close()
	sc, err := sftp.NewClient(cli)
	if err != nil {
		return err
	}
	defer sc.Close()
	return fn(sc)
}

func (s *SFTP) sshConfig() (*ssh.ClientConfig, string, error) {
	if s.User == "" {
		return nil, "", fmt.Errorf("PANEL_SFTP_USER required")
	}
	var auths []ssh.AuthMethod
	if len(s.KeyPEM) > 0 {
		signer, err := ssh.ParsePrivateKey(s.KeyPEM)
		if err != nil {
			return nil, "", fmt.Errorf("PANEL_SFTP_KEY: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if s.Password != "" {
		auths = append(auths, ssh.Password(s.Password))
	}
	if len(auths) == 0 {
		return nil, "", fmt.Errorf("PANEL_SFTP_PASSWORD or PANEL_SFTP_KEY required")
	}
	if strings.TrimSpace(s.HostKey) == "" {
		return nil, "", fmt.Errorf("PANEL_SFTP_HOST_KEY (SHA256 fingerprint) required")
	}
	addr := s.Host
	if !strings.Contains(addr, ":") {
		addr += ":22"
	}
	return &ssh.ClientConfig{
		User:            s.User,
		Auth:            auths,
		HostKeyCallback: fixedHostKey(s.HostKey),
		Timeout:         s.timeout(),
	}, addr, nil
}

func (s *SFTP) timeout() time.Duration {
	if s.DialTimeout > 0 {
		return s.DialTimeout
	}
	return 20 * time.Second
}

func fixedHostKey(want string) ssh.HostKeyCallback {
	want = normalizeFP(want)
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		got := normalizeFP(ssh.FingerprintSHA256(key))
		if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
			return fmt.Errorf("ssh host key mismatch")
		}
		return nil
	}
}

func normalizeFP(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "SHA256:")
	return strings.TrimRight(s, "=")
}

var _ Repository = (*SFTP)(nil)
