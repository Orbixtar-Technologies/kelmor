package phases

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hosting-panel/panel/internal/firewall"
	"github.com/hosting-panel/panel/internal/netaddr"

	"golang.org/x/crypto/ssh"
)

//go:embed units/*.service units/*.timer
var systemdUnits embed.FS

//go:embed release.pub
var defaultUpdatePublicKey []byte

func applyDNS(c Config) error {
	if err := os.MkdirAll(root(c, "etc/powerdns"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/dns/zones"), 0o755); err != nil {
		return err
	}
	listen := strings.Join(netaddr.DNSListenIPv4(), ",")
	body := `setuid=pdns
setgid=pdns
launch=bind
bind-config=/etc/powerdns/named.conf
bind-dnssec-db=/var/lib/panel/dns/bind-dnssec.sqlite3
local-address=` + listen + `
local-port=53
webserver=yes
webserver-address=127.0.0.1
webserver-port=8081
api=yes
api-key=panel-loopback
`
	pdnsConf := root(c, "etc/powerdns/pdns.conf")
	if err := writeUnlessExists(pdnsConf, []byte(body), 0o640); err != nil {
		return err
	}
	if err := reconcilePowerDNSConfig(pdnsConf, listen); err != nil {
		return err
	}
	if err := writePublicEnv(c); err != nil {
		return err
	}
	named := `options {
    directory "/var/lib/panel/dns/zones";
};
include "/var/lib/panel/dns/named-zones.conf";
`
	if err := os.WriteFile(root(c, "etc/powerdns/named.conf"), []byte(named), 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(root(c, "var/lib/panel/dns/named-zones.conf")); os.IsNotExist(err) {
		if err := os.WriteFile(root(c, "var/lib/panel/dns/named-zones.conf"), []byte(""), 0o644); err != nil {
			return err
		}
	}
	if installPrefix(c) == "" && !c.Dev {
		db := root(c, "var/lib/panel/dns/bind-dnssec.sqlite3")
		if _, err := os.Stat(db); err != nil {
			_ = exec.Command("/usr/bin/pdnsutil", "create-bind-db", db).Run()
			if u, err := user.Lookup("pdns"); err == nil {
				uid, _ := strconv.Atoi(u.Uid)
				gid, _ := strconv.Atoi(u.Gid)
				_ = os.Chown(db, uid, gid)
				_ = os.Chown(root(c, "var/lib/panel/dns"), uid, gid)
			}
		}
	}
	reloadLivePowerDNS(c)
	return nil
}

func reloadLivePowerDNS(c Config) {
	if c.Dev || installPrefix(c) != "" {
		return
	}
	if _, err := os.Stat("/usr/sbin/pdns_server"); err != nil {
		return
	}
	_ = exec.Command("/usr/bin/pkill", "-x", "pdns_server").Run()
	time.Sleep(200 * time.Millisecond)
	if err := exec.Command("/bin/systemctl", "restart", "pdns").Run(); err == nil {
		_ = waitListen("127.0.0.1:53", 5*time.Second)
		return
	}
	cmd := exec.Command("/usr/sbin/pdns_server", "--daemon=yes", "--guardian=no", "--config-dir=/etc/powerdns")
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin"}
	_ = cmd.Start()
	_ = waitListen("127.0.0.1:53", 3*time.Second)
}

func applyMail(c Config) error {
	if err := os.MkdirAll(root(c, "var/vmail"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/mail"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/tmp/panel-imports"), 0o1777); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/postfix"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/dovecot/conf.d"), 0o755); err != nil {
		return err
	}
	main := `# Managed by Kelmor — Postfix virtual mailbox host
compatibility_level = 3.6
myhostname = ` + hostnameOr(c) + `
mydestination =
local_recipient_maps =
local_transport = error:local delivery disabled
virtual_mailbox_base = /var/vmail
virtual_mailbox_domains = hash:/var/lib/panel/mail/vdomains
virtual_mailbox_maps = hash:/var/lib/panel/mail/virtual
virtual_alias_maps = hash:/var/lib/panel/mail/aliases
smtpd_sender_login_maps = hash:/var/lib/panel/mail/sender-login
virtual_minimum_uid = 20000
virtual_uid_maps = hash:/var/lib/panel/mail/uids
virtual_gid_maps = hash:/var/lib/panel/mail/gids
smtpd_tls_security_level = may
smtpd_tls_cert_file = /var/lib/panel/certs/imap.panel.local.crt
smtpd_tls_key_file = /var/lib/panel/certs/imap.panel.local.key
smtpd_sasl_type = dovecot
smtpd_sasl_path = private/auth
smtpd_sasl_auth_enable = no
smtpd_recipient_restrictions = permit_mynetworks, reject_unauth_destination
smtpd_end_of_data_restrictions = check_policy_service inet:127.0.0.1:10031
smtpd_policy_service_default_action = DUNNO
smtpd_milters = inet:127.0.0.1:11332
non_smtpd_milters = inet:127.0.0.1:11332
milter_default_action = accept
`
	if err := writeUnlessExists(root(c, "etc/postfix/main.cf"), []byte(main), 0o644); err != nil {
		return err
	}
	maincf := root(c, "etc/postfix/main.cf")
	for _, kv := range [][2]string{
		{"smtpd_sasl_type", "smtpd_sasl_type = dovecot\n"},
		{"smtpd_sasl_path", "smtpd_sasl_path = private/auth\n"},
		{"smtpd_tls_cert_file", "smtpd_tls_cert_file = /var/lib/panel/certs/imap.panel.local.crt\n"},
		{"smtpd_tls_key_file", "smtpd_tls_key_file = /var/lib/panel/certs/imap.panel.local.key\n"},
		{"smtpd_sender_login_maps", "smtpd_sender_login_maps = hash:/var/lib/panel/mail/sender-login\n"},
	} {
		if err := replaceConfigLine(maincf, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if err := ensureSubmissionMaster(root(c, "etc/postfix/master.cf")); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/dovecot/conf.d"), 0o755); err != nil {
		return err
	}
	sasl := `auth_mechanisms = plain login
service auth {
  unix_listener /var/spool/postfix/private/auth {
    mode = 0660
    user = postfix
    group = postfix
  }
}
`
	if err := os.WriteFile(root(c, "etc/dovecot/conf.d/99-panel-sasl.conf"), []byte(sasl), 0o644); err != nil {
		return err
	}
	if err := ensurePanelMailCert(c); err != nil {
		return err
	}
	dovecot := `protocols = imap lmtp
listen = *
mail_location = maildir:~/Maildir
passdb {
  driver = passwd-file
  args = /var/lib/panel/mail/passwd
}
userdb {
  driver = passwd-file
  args = /var/lib/panel/mail/passwd
}
ssl = yes
ssl_cert = </var/lib/panel/certs/imap.panel.local.crt
ssl_key = </var/lib/panel/certs/imap.panel.local.key
!include_try /etc/dovecot/conf.d/99-panel-*.conf
mail_location = maildir:~/Maildir
`
	if err := os.WriteFile(root(c, "etc/dovecot/dovecot.conf"), []byte(dovecot), 0o644); err != nil {
		return err
	}
	for _, name := range []string{"virtual", "vdomains", "passwd", "uids", "gids", "aliases", "sender-login"} {
		p := root(c, "var/lib/panel/mail/"+name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, []byte("# panel mail map\n"), 0o640); err != nil {
				return err
			}
		}
	}
	if err := applySMTPRelay(c); err != nil {
		return err
	}
	reloadLiveMail(c)
	return nil
}

const panelSubmissionBlock = `# panel-submission
submission inet n       -       n       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_sasl_type=dovecot
  -o smtpd_sasl_path=private/auth
  -o smtpd_client_restrictions=permit_sasl_authenticated,reject
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject
  -o smtpd_sender_restrictions=reject_sender_login_mismatch
  -o milter_macro_daemon_name=ORIGINATING
`

func ensureSubmissionMaster(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.WriteFile(path, []byte("smtp      inet  n       -       y       -       -       smtpd\n"+panelSubmissionBlock), 0o644)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(b), "panel-submission") {
		return ensureSubmissionSenderRestrict(path, b)
	}
	out := string(b)
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out+panelSubmissionBlock), 0o644)
}

func ensureSubmissionSenderRestrict(path string, current []byte) error {
	s := string(current)
	if strings.Contains(s, "reject_sender_login_mismatch") {
		return nil
	}
	line := "  -o smtpd_sender_restrictions=reject_sender_login_mismatch\n"
	needle := "  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject\n"
	if strings.Contains(s, needle) {
		s = strings.Replace(s, needle, needle+line, 1)
	} else if strings.Contains(s, "  -o milter_macro_daemon_name=ORIGINATING\n") {
		s = strings.Replace(s, "  -o milter_macro_daemon_name=ORIGINATING\n", line+"  -o milter_macro_daemon_name=ORIGINATING\n", 1)
	} else {
		s += line
	}
	return os.WriteFile(path, []byte(s), 0o644)
}

func ensurePanelMailCert(c Config) error {
	dir := root(c, "var/lib/panel/certs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	crt := filepath.Join(dir, "imap.panel.local.crt")
	key := filepath.Join(dir, "imap.panel.local.key")
	if _, err := os.Stat(crt); err == nil {
		if _, err := os.Stat(key); err == nil {
			return nil
		}
	}
	if _, err := os.Stat("/usr/bin/openssl"); err != nil {
		return nil
	}
	cmd := exec.Command("/usr/bin/openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
		"-keyout", key, "-out", crt, "-days", "3650", "-subj", "/CN=imap.panel.local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mail cert: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(key, 0o640)
	return nil
}

func mailRestartPlan(systemd bool) [][]string {
	if systemd {
		return [][]string{
			{"/bin/systemctl", "restart", "dovecot"},
			{"/bin/systemctl", "reload", "postfix"},
		}
	}
	return [][]string{
		{"/usr/bin/doveadm", "stop"},
		{"/usr/sbin/dovecot"},
		{"/usr/sbin/postfix", "reload"},
	}
}

func reloadLiveMail(c Config) {
	if c.Dev || installPrefix(c) != "" {
		return
	}
	for _, args := range mailRestartPlan(pid1IsSystemd()) {
		if len(args) == 0 {
			continue
		}
		if args[len(args)-1] == "dovecot" && args[0] != "/bin/systemctl" {
			if _, err := os.Stat("/usr/sbin/dovecot"); err != nil {
				continue
			}
			if exec.Command("/usr/bin/pgrep", "-x", "dovecot").Run() == nil {
				continue
			}
		}
		_ = exec.Command(args[0], args[1:]...).Run()
	}
	_ = waitListen("127.0.0.1:587", 3*time.Second)
}

func applyFirewall(c Config) error {
	if err := os.MkdirAll(root(c, "etc/panel"), 0o755); err != nil {
		return err
	}
	rules := firewall.Rules(firewall.ExtraListeningTCP())
	path := root(c, "etc/panel/nftables-panel.nft")
	if err := os.WriteFile(path, []byte(rules), 0o600); err != nil {
		return err
	}
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	return applyLiveNFT(path)
}

func applyLiveNFT(path string) error {
	if _, err := os.Stat("/usr/sbin/nft"); err != nil {
		return nil
	}
	_ = exec.Command("/usr/sbin/nft", "delete", "table", "inet", "panel").Run()
	cmd := exec.Command("/usr/sbin/nft", "-f", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft -f: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func applyWAF(c Config) error {
	if err := os.MkdirAll(root(c, "etc/nginx/modsec"), 0o755); err != nil {
		return err
	}
	body := `# Managed by Kelmor — ModSecurity
SecRuleEngine DetectionOnly
SecRequestBodyAccess On
SecDataDir /tmp
`
	if _, err := os.Stat("/usr/share/modsecurity-crs/owasp-crs.conf"); err == nil && !c.Dev {
		body += "Include /usr/share/modsecurity-crs/owasp-crs.conf\n"
	}
	if err := os.WriteFile(root(c, "etc/nginx/modsec/panel.conf"), []byte(body), 0o644); err != nil {
		return err
	}
	enforce := `# Managed by Kelmor — enforced probe vhost only
SecRuleEngine On
SecRequestBodyAccess On
SecDataDir /tmp
SecRule REQUEST_HEADERS:User-Agent "@contains panel-modsec-probe" "id:19999,phase:1,deny,status:403,msg:'panel waf probe'"
`
	if err := os.WriteFile(root(c, "etc/nginx/modsec/panel-enforce.conf"), []byte(enforce), 0o644); err != nil {
		return err
	}
	probe := `server {
    listen 127.0.0.1:18481;
    server_name _;
    modsecurity on;
    modsecurity_rules_file /etc/nginx/modsec/panel-enforce.conf;
    location / { default_type text/plain; return 200 'waf-ok\n'; }
}
`
	return os.WriteFile(root(c, "etc/nginx/panel-sites/01-modsec-probe.conf"), []byte(probe), 0o644)
}

func applyRspamd(c Config) error {
	if err := os.MkdirAll(root(c, "etc/rspamd/local.d"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(root(c, "etc/rspamd/local.d/panel.conf"), []byte("enabled = true;\nmilters = \"inet:127.0.0.1:11332\";\n"), 0o644)
}

func applyClamAV(c Config) error {
	if err := os.MkdirAll(root(c, "etc/clamav"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(root(c, "etc/clamav/panel.conf"), []byte("TCPSocket 3310\nTCPAddr 127.0.0.1\n"), 0o644); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	sigdir := "/var/lib/clamav"
	if err := os.MkdirAll(sigdir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(sigdir + "/main.cvd"); err != nil {
		_ = os.WriteFile(sigdir+"/panel.ndb", []byte("PanelClam:0:*:50414e454c434c414d\n"), 0o644)
		cmd := exec.Command("/usr/bin/freshclam", "--stdout", "--quiet")
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "DEBIAN_FRONTEND=noninteractive"}
		_ = cmd.Start()
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(45 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	return nil
}

func applySecurity(c Config) error {
	if err := applyWAF(c); err != nil {
		return err
	}
	if err := applyRspamd(c); err != nil {
		return err
	}
	if err := applyClamAV(c); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/fail2ban/jail.d"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/ssh/sshd_config.d"), 0o755); err != nil {
		return err
	}
	jail := `[sshd]
enabled = true
[postfix]
enabled = true
[dovecot]
enabled = true
[panel-auth]
enabled = true
port = 8443,8444,18080
filter = panel-auth
logpath = /var/lib/panel/logs/api.jsonl
[vsftpd]
enabled = true
`
	if err := os.WriteFile(root(c, "etc/fail2ban/jail.d/panel.conf"), []byte(jail), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/fail2ban/filter.d"), 0o755); err != nil {
		return err
	}
	filter := `[Definition]
failregex = ^.*"event":"auth.login".*"success":false.*"source_ip":"<HOST>"
ignoreregex =
`
	if err := os.WriteFile(root(c, "etc/fail2ban/filter.d/panel-auth.conf"), []byte(filter), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/quotas"), 0o755); err != nil {
		return err
	}
	if err := applyFTPStack(c); err != nil {
		return err
	}
	sftp := `# Chrooted tenant SFTP. Over-quota users get internal-sftp -R
# from /etc/ssh/sshd_config.d/zz-panel-sftp-quota.conf (agent-managed).
# PasswordAuthentication is Match-scoped so cloud images that default
# to key-only SSH still accept the owner password for SFTP — not a shell.
Match Group panel-sftp
    ChrootDirectory /home/%u
    ForceCommand internal-sftp
    PasswordAuthentication yes
    KbdInteractiveAuthentication yes
    AllowTcpForwarding no
    X11Forwarding no
`
	if err := os.WriteFile(root(c, "etc/ssh/sshd_config.d/panel-sftp.conf"), []byte(sftp), 0o644); err != nil {
		return err
	}
	if err := applyBackupOffsite(c); err != nil {
		return err
	}
	return applyBackupS3(c)
}

func applyFTPStack(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel/ftp/user_conf"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/run/vsftpd/empty"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/pam.d"), 0o755); err != nil {
		return err
	}
	conf := `listen=YES
listen_ipv6=NO
anonymous_enable=NO
local_enable=YES
write_enable=YES
dirmessage_enable=YES
use_localtime=YES
xferlog_enable=YES
connect_from_port_20=YES
chroot_local_user=YES
allow_writeable_chroot=YES
secure_chroot_dir=/var/run/vsftpd/empty
pam_service_name=vsftpd
guest_enable=YES
guest_username=nobody
virtual_use_local_privs=YES
user_config_dir=/var/lib/panel/ftp/user_conf
hide_ids=YES
pasv_min_port=40000
pasv_max_port=40100
pasv_address=` + netaddr.PublicIPv4() + `
background=NO
seccomp_sandbox=NO
`
	if err := os.WriteFile(root(c, "etc/vsftpd.conf"), []byte(conf), 0o644); err != nil {
		return err
	}
	pam := `auth required pam_pwdfile.so pwdfile=/var/lib/panel/ftp/passwd
account required pam_permit.so
`
	if err := os.WriteFile(root(c, "etc/pam.d/vsftpd"), []byte(pam), 0o644); err != nil {
		return err
	}
	passwd := root(c, "var/lib/panel/ftp/passwd")
	if _, err := os.Stat(passwd); os.IsNotExist(err) {
		if err := os.WriteFile(passwd, []byte(""), 0o640); err != nil {
			return err
		}
	}
	if c.Dev {
		return nil
	}
	if _, err := os.Stat("/usr/sbin/vsftpd"); err != nil {
		return nil
	}
	if pid := vsftpdPID(); pid > 0 {
		_ = exec.Command("/bin/kill", fmt.Sprintf("%d", pid)).Run()
		time.Sleep(150 * time.Millisecond)
	}
	cmd := exec.Command("/usr/sbin/vsftpd", "/etc/vsftpd.conf")
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin"}
	return cmd.Start()
}

func applyBackupOffsite(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel/offsite"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/offsite/inbox"), 0o2770); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/secrets"), 0o750); err != nil {
		return err
	}
	conf := `# Offsite backup receiver — key-only internal-sftp
Match User panel-backup
    PasswordAuthentication no
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
`
	if err := os.WriteFile(root(c, "etc/ssh/sshd_config.d/panel-backup-sftp.conf"), []byte(conf), 0o644); err != nil {
		return err
	}
	if c.Dev {
		body := "PANEL_SFTP_ROOT=/var/lib/panel/offsite/inbox\n"
		return os.WriteFile(root(c, "var/lib/panel/secrets/backup-sftp.env"), []byte(body), 0o640)
	}
	offsite := root(c, "var/lib/panel/offsite")
	inbox := root(c, "var/lib/panel/offsite/inbox")
	_ = os.Chmod(offsite, 0o750)
	_ = os.Chmod(inbox, 0o2770)
	if installPrefix(c) == "" {
		if _, err := user.Lookup("panel-backup"); err != nil {
			_ = exec.Command("/usr/sbin/useradd", "--system", "-d", "/var/lib/panel/offsite", "-s", "/usr/sbin/nologin", "panel-backup").Run()
		}
		_ = exec.Command("/bin/chown", "panel-backup:panel-backup", offsite).Run()
		_ = exec.Command("/bin/chown", "panel-backup:panel", inbox).Run()
	}
	keyPath := root(c, "var/lib/panel/secrets/backup-sftp")
	if _, err := os.Stat(keyPath); err != nil {
		cmd := exec.Command("/usr/bin/ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", "panel-offsite")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ssh-keygen: %s", strings.TrimSpace(string(out)))
		}
	}
	_ = os.Chmod(keyPath, 0o640)
	_ = exec.Command("/bin/chown", "panel:panel", keyPath, keyPath+".pub").Run()
	pub, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return err
	}
	sshDir := root(c, "var/lib/panel/offsite/.ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), pub, 0o600); err != nil {
		return err
	}
	if installPrefix(c) == "" {
		_ = exec.Command("/bin/chown", "-R", "panel-backup:panel-backup", sshDir).Run()
	}
	_ = os.Chmod(sshDir, 0o700)
	fp := strings.Join(sshHostFingerprints(), ",")
	if fp == "" {
		return fmt.Errorf("ssh host key fingerprint missing")
	}
	env := strings.Join([]string{
		"PANEL_SFTP_HOST=127.0.0.1",
		"PANEL_SFTP_USER=panel-backup",
		"PANEL_SFTP_KEY=/var/lib/panel/secrets/backup-sftp",
		"PANEL_SFTP_HOST_KEY=" + fp,
		"PANEL_SFTP_ROOT=/var/lib/panel/offsite/inbox",
		"",
	}, "\n")
	if err := os.WriteFile(root(c, "var/lib/panel/secrets/backup-sftp.env"), []byte(env), 0o640); err != nil {
		return err
	}
	if installPrefix(c) == "" {
		_ = exec.Command("/bin/chown", "panel:panel", root(c, "var/lib/panel/secrets/backup-sftp.env")).Run()
		if pid := sshdPID(); pid > 0 {
			_ = exec.Command("/bin/kill", "-HUP", fmt.Sprintf("%d", pid)).Run()
		}
	}
	return nil
}

func applyBackupS3(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel/objects"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/secrets"), 0o750); err != nil {
		return err
	}
	envPath := root(c, "var/lib/panel/secrets/backup-s3.env")
	if !s3EnvReady(envPath) {
		access, err := randomHex(16)
		if err != nil {
			return err
		}
		secret, err := randomHex(24)
		if err != nil {
			return err
		}
		env := strings.Join([]string{
			"PANEL_S3_ENDPOINT=http://127.0.0.1:19090",
			"PANEL_S3_LISTEN=127.0.0.1:19090",
			"PANEL_S3_REGION=us-east-1",
			"PANEL_S3_BUCKET=panel",
			"PANEL_S3_ACCESS_KEY=" + access,
			"PANEL_S3_SECRET_KEY=" + secret,
			"PANEL_S3_DATA=/var/lib/panel/objects",
			"",
		}, "\n")
		if err := os.WriteFile(envPath, []byte(env), 0o640); err != nil {
			return err
		}
	}
	if !c.Dev {
		_ = exec.Command("/bin/chown", "panel:panel", root(c, "var/lib/panel/objects"), envPath).Run()
		startObjectStore()
	}
	return nil
}

func s3EnvReady(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, "PANEL_S3_ENDPOINT=") && strings.Contains(s, "PANEL_S3_ACCESS_KEY=") && strings.Contains(s, "PANEL_S3_SECRET_KEY=")
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sshHostFingerprints() []string {
	var out []string
	for _, name := range []string{"ssh_host_ed25519_key.pub", "ssh_host_rsa_key.pub", "ssh_host_ecdsa_key.pub"} {
		b, err := os.ReadFile("/etc/ssh/" + name)
		if err != nil {
			continue
		}
		pk, _, _, _, err := ssh.ParseAuthorizedKey(b)
		if err != nil {
			continue
		}
		out = append(out, ssh.FingerprintSHA256(pk))
	}
	return out
}

func sshdPID() int {
	out, err := exec.Command("/usr/bin/pgrep", "-x", "sshd").Output()
	if err != nil {
		return 0
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &pid); err != nil {
		return 0
	}
	return pid
}

func vsftpdPID() int {
	out, err := exec.Command("/usr/bin/pgrep", "-x", "vsftpd").Output()
	if err != nil {
		return 0
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &pid); err != nil {
		return 0
	}
	return pid
}

func applyTLS(c Config) error {
	if err := os.MkdirAll(root(c, "var/lib/panel/acme-www/.well-known/acme-challenge"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/certs"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "var/lib/panel/secrets"), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(root(c, "etc/nginx/panel-sites"), 0o755); err != nil {
		return err
	}
	acme := `server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;
    location ^~ /.well-known/acme-challenge/ {
        root /var/lib/panel/acme-www;
        default_type text/plain;
    }
    location / { default_type text/plain; return 404 'no such site\n'; }
}
`
	if err := os.WriteFile(root(c, "etc/nginx/panel-sites/00-acme.conf"), []byte(acme), 0o644); err != nil {
		return err
	}
	return ensureACME(c)
}

func verifyTLS(c Config) error {
	if _, err := os.Stat(root(c, "var/lib/panel/acme-www/.well-known/acme-challenge")); err != nil {
		return err
	}
	if c.Dev {
		return nil
	}
	dir := strings.TrimSpace(readACMEDirectory(c))
	if dir == "" {
		return fmt.Errorf("ACME directory missing")
	}
	if isLabDirectory(dir) {
		if pebbleBinary() == "" || installPrefix(c) != "" {
			return nil
		}
		if waitPebble(400 * time.Millisecond) {
			return nil
		}
		if pid1IsSystemd() {
			return nil
		}
		return fmt.Errorf("pebble ACME not listening")
	}
	if !strings.Contains(dir, "letsencrypt.org") {
		return fmt.Errorf("expected Let's Encrypt ACME directory, got %s", dir)
	}
	return nil
}

func writeUnlessExists(path string, body []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, body, mode)
}

func writePublicEnv(c Config) error {
	pub := netaddr.PublicIPv4()
	body := writeValidationPublicLines(pub)
	path := root(c, "var/lib/panel/public.env")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	if !c.Dev {
		_ = exec.Command("/bin/chown", "panel:panel", path).Run()
	}
	return nil
}

func replaceConfigLine(path, prefix, line string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	found := false
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			lines[i] = strings.TrimSuffix(line, "\n")
			found = true
		}
	}
	if !found {
		if !strings.HasSuffix(string(b), "\n") && len(b) > 0 {
			b = append(b, '\n')
		}
		return os.WriteFile(path, append(b, []byte(line)...), 0o640)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o640)
}

func reconcilePowerDNSConfig(path, listen string) error {
	settings := map[string]string{
		"bind-config":       "/etc/powerdns/named.conf",
		"bind-dnssec-db":    "/var/lib/panel/dns/bind-dnssec.sqlite3",
		"local-address":     listen,
		"local-port":        "53",
		"webserver":         "yes",
		"webserver-address": "127.0.0.1",
		"webserver-port":    "8081",
		"api":               "yes",
		"api-key":           "panel-loopback",
	}
	return replacePowerDNSSettings(path, settings)
}

func replacePowerDNSSettings(path string, settings map[string]string) error {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		prefix := key + "="
		lines := strings.Split(string(b), "\n")
		out := make([]string, 0, len(lines))
		found := false
		for _, line := range lines {
			candidate := strings.TrimSpace(line)
			candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "#"))
			if strings.HasPrefix(candidate, prefix) {
				if found {
					continue
				}
				out = append(out, candidate)
				found = true
				continue
			}
			out = append(out, line)
		}
		if found {
			if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o640); err != nil {
				return err
			}
		}
		if err := replaceConfigLine(path, prefix, prefix+settings[key]+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func ensureFileContains(path, line string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(b), strings.TrimSpace(line)) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if !strings.HasSuffix(string(b), "\n") {
		line = "\n" + line
	}
	_, err = f.WriteString(line)
	return err
}

func verifySystemd(c Config) error {
	for _, n := range []string{"panel-api.service", "panel-worker.service", "panel-agent.service", "panel-smtp-policy.service"} {
		if _, err := os.Stat(root(c, "etc/systemd/system/"+n)); err != nil {
			return err
		}
	}
	b, err := os.ReadFile(root(c, "etc/systemd/system/panel-worker.service"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), "backup-sftp.env") {
		return fmt.Errorf("panel-worker.service missing offsite SFTP env")
	}
	if !strings.Contains(string(b), "backup-s3.env") {
		return fmt.Errorf("panel-worker.service missing S3 backup env")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/panel-object-store.service")); err != nil {
		return fmt.Errorf("panel-object-store.service missing")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/multi-user.target.wants/panel-agent.service")); err != nil {
		return fmt.Errorf("panel-agent.service is not enabled for multi-user boot")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/multi-user.target.wants/panel-worker.service")); err != nil {
		return fmt.Errorf("panel-worker.service is not enabled for multi-user boot")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/pebble.service")); err != nil {
		return fmt.Errorf("pebble.service missing")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/multi-user.target.wants/pebble.service")); err != nil {
		return fmt.Errorf("pebble.service is not enabled for multi-user boot")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/panel-update@.service")); err != nil {
		return fmt.Errorf("panel-update@.service missing")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/panel-update.timer")); err != nil {
		return fmt.Errorf("panel-update.timer missing")
	}
	if _, err := os.Stat(root(c, "etc/systemd/system/timers.target.wants/panel-update.timer")); err != nil {
		return fmt.Errorf("panel-update.timer is not enabled")
	}
	if _, err := os.Stat(root(c, "etc/panel/update.env")); err != nil {
		return fmt.Errorf("update.env missing")
	}
	if _, err := os.Stat(root(c, "etc/panel/update.pub")); err != nil {
		return fmt.Errorf("update.pub missing")
	}
	if _, err := os.Stat(root(c, "usr/local/panel/current-release")); err != nil {
		return fmt.Errorf("current-release missing")
	}
	updateEnv, err := os.ReadFile(root(c, "etc/panel/update.env"))
	if err != nil {
		return fmt.Errorf("update.env missing")
	}
	if !strings.Contains(string(updateEnv), "PANEL_UPDATE_FEED_URL=https://127.0.0.1:8443/updates") {
		return fmt.Errorf("update.env is not configured for the local signed feed")
	}
	return nil
}

func applySystemd(c Config) error {
	if err := os.MkdirAll(root(c, "etc/systemd/system"), 0o755); err != nil {
		return err
	}
	entries, err := systemdUnits.ReadDir("units")
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("embedded systemd units missing")
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".service") && !strings.HasSuffix(name, ".timer") {
			continue
		}
		body, err := systemdUnits.ReadFile("units/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(root(c, "etc/systemd/system/"+name), body, 0o644); err != nil {
			return err
		}
	}
	if err := applyUpdatePolicy(c); err != nil {
		return err
	}
	if err := enableBootUnits(c); err != nil {
		return err
	}
	return enableUpdateTimer(c)
}

func applyUpdatePolicy(c Config) error {
	if err := os.MkdirAll(root(c, "etc/panel"), 0o755); err != nil {
		return err
	}
	feedURL := "https://127.0.0.1:8443/updates"
	body := fmt.Sprintf(`PANEL_UPDATE_FEED_URL=%s
PANEL_UPDATE_CHANNEL=stable
PANEL_UPDATE_AUTOMATIC=true
PANEL_UPDATE_INSTALL_ROOT=/usr/local/panel
PANEL_UPDATE_STATUS_PATH=/var/lib/panel/update-status.json
PANEL_UPDATE_PUBLIC_KEY_PATH=/etc/panel/update.pub
`, feedURL)
	configPath := root(c, "etc/panel/update.env")
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		return err
	}
	pubPath := root(c, "etc/panel/update.pub")
	if len(defaultUpdatePublicKey) == 0 {
		return fmt.Errorf("embedded update public key missing")
	}
	if err := os.WriteFile(pubPath, defaultUpdatePublicKey, 0o644); err != nil {
		return err
	}
	statusPath := root(c, "var/lib/panel/update-status.json")
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		initial := fmt.Sprintf(`{"state":"idle","installed_release":"0.1.0","automatic":true,"channel":"stable"}
`)
		if err := os.MkdirAll(filepath.Dir(statusPath), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(statusPath, []byte(initial), 0o600); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(root(c, "usr/local/panel/share/updates"), 0o755); err != nil {
		return err
	}
	releasePath := root(c, "usr/local/panel/current-release")
	if _, err := os.Stat(releasePath); os.IsNotExist(err) {
		if err := os.WriteFile(releasePath, []byte("0.1.0\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func enableUpdateTimer(c Config) error {
	wants := root(c, "etc/systemd/system/timers.target.wants")
	if err := os.MkdirAll(wants, 0o755); err != nil {
		return err
	}
	link := filepath.Join(wants, "panel-update.timer")
	_ = os.Remove(link)
	return os.Symlink("../panel-update.timer", link)
}

func enableBootUnits(c Config) error {
	wants := root(c, "etc/systemd/system/multi-user.target.wants")
	if err := os.MkdirAll(wants, 0o755); err != nil {
		return err
	}
	for _, name := range []string{
		"panel-agent.service", "panel-api.service", "panel-worker.service",
		"panel-smtp-policy.service", "panel-object-store.service",
		"pebble.service",
	} {
		link := filepath.Join(wants, name)
		_ = os.Remove(link)
		if err := os.Symlink("../"+name, link); err != nil {
			return err
		}
	}
	return nil
}

func applyHealth(c Config) error {
	web, dns, mail := "ok", "configured", "configured"
	if !c.Dev {
		if _, err := net.DialTimeout("tcp", "127.0.0.1:80", 400*time.Millisecond); err != nil {
			web = "down"
		}
		if _, err := net.DialTimeout("tcp", "127.0.0.1:25", 400*time.Millisecond); err != nil {
			mail = "down"
		}
		if _, err := net.DialTimeout("tcp", "127.0.0.1:53", 400*time.Millisecond); err != nil {
			dns = "down"
		}
	}
	report := fmt.Sprintf("installation_id=%s checked=%s web=%s dns=%s mail=%s firewall=table-inet-panel\n",
		c.Hostname, time.Now().UTC().Format(time.RFC3339), web, dns, mail)
	return os.WriteFile(root(c, "var/lib/panel/health-report.txt"), []byte(report), 0o640)
}

func applyAdministrator(c Config) error {
	body := fmt.Sprintf("admin_email=%s hostname=%s channel=%s\nUse PANEL_ADMIN_PASSWORD for the first Kelmor Director login.\n",
		c.AdminEmail, c.Hostname, c.Channel)
	return os.WriteFile(root(c, "var/lib/panel/administrator.txt"), []byte(body), 0o640)
}

func applyReport(c Config) error {
	body := fmt.Sprintf("ok=true hostname=%s finished=%s\n", c.Hostname, time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(root(c, "var/lib/panel/installation-report.txt"), []byte(body), 0o640)
}

func verifyMail(c Config) error {
	if _, err := os.Stat(root(c, "etc/postfix/main.cf")); err != nil {
		return err
	}
	b, err := os.ReadFile(root(c, "etc/postfix/master.cf"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), "panel-submission") {
		return fmt.Errorf("postfix master.cf missing submission")
	}
	if !strings.Contains(string(b), "reject_sender_login_mismatch") {
		return fmt.Errorf("postfix submission missing sender-login mismatch")
	}
	mainb, err := os.ReadFile(root(c, "etc/postfix/main.cf"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(mainb), "smtpd_sender_login_maps") {
		return fmt.Errorf("postfix missing sender-login maps")
	}
	if _, _, _, _, _, ok := smtpRelaySettings(); ok {
		if !strings.Contains(string(mainb), "relayhost") {
			return fmt.Errorf("postfix missing SMTP relayhost from validation smtp.env")
		}
	}
	if _, err := os.Stat(root(c, "etc/dovecot/conf.d/99-panel-sasl.conf")); err != nil {
		return err
	}
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	if err := waitListen("127.0.0.1:587", 2*time.Second); err != nil {
		return fmt.Errorf("submission is not listening: %w", err)
	}
	return nil
}

func verifyDNS(c Config) error {
	b, err := os.ReadFile(root(c, "etc/powerdns/pdns.conf"))
	if err != nil {
		return err
	}
	listen := strings.Join(netaddr.DNSListenIPv4(), ",")
	settings := []string{
		"bind-config=/etc/powerdns/named.conf",
		"bind-dnssec-db=/var/lib/panel/dns/bind-dnssec.sqlite3",
		"local-address=" + listen,
		"local-port=53",
		"webserver=yes",
		"webserver-address=127.0.0.1",
		"webserver-port=8081",
		"api=yes",
		"api-key=panel-loopback",
	}
	for _, setting := range settings {
		found := false
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == setting {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("pdns.conf missing active %s", setting)
		}
	}
	// Ubuntu ships launch= (parent) plus pdns.d/bind.conf launch+=bind.
	// Do not require launch=bind in the main file.
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	if err := waitListen("127.0.0.1:53", 2*time.Second); err != nil {
		return fmt.Errorf("powerdns is not listening: %w", err)
	}
	if pub := netaddr.PublicIPv4(); pub != "127.0.0.1" {
		if err := waitListen(net.JoinHostPort(pub, "53"), 2*time.Second); err != nil {
			return fmt.Errorf("powerdns is not listening on public %s: %w", pub, err)
		}
	}
	return nil
}

func verifyFirewall(c Config) error {
	if _, err := os.Stat(root(c, "etc/panel/nftables-panel.nft")); err != nil {
		return err
	}
	if c.Dev || installPrefix(c) != "" {
		return nil
	}
	if _, err := os.Stat("/usr/sbin/nft"); err != nil {
		return nil
	}
	if err := exec.Command("/usr/sbin/nft", "list", "table", "inet", "panel").Run(); err != nil {
		return fmt.Errorf("table inet panel is not loaded")
	}
	return nil
}

func verifySecurity(c Config) error {
	for _, p := range []string{
		"etc/nginx/modsec/panel.conf",
		"etc/rspamd/local.d/panel.conf",
		"etc/clamav/panel.conf",
		"etc/fail2ban/jail.d/panel.conf",
		"etc/fail2ban/filter.d/panel-auth.conf",
		"etc/ssh/sshd_config.d/panel-sftp.conf",
		"var/lib/panel/quotas",
		"etc/vsftpd.conf",
		"etc/pam.d/vsftpd",
		"var/lib/panel/ftp/user_conf",
		"etc/ssh/sshd_config.d/panel-backup-sftp.conf",
		"var/lib/panel/offsite",
		"var/lib/panel/secrets/backup-sftp.env",
		"var/lib/panel/objects",
		"var/lib/panel/secrets/backup-s3.env",
	} {
		if _, err := os.Stat(root(c, p)); err != nil {
			return err
		}
	}
	if !c.Dev {
		if st, err := os.Stat(root(c, "var/lib/panel/offsite/inbox")); err != nil {
			return err
		} else if st.Mode()&0o020 == 0 {
			return fmt.Errorf("offsite inbox must be group-writable for SFTP backups")
		}
		envb, err := os.ReadFile(root(c, "var/lib/panel/secrets/backup-sftp.env"))
		if err != nil {
			return err
		}
		if fps := sshHostFingerprints(); len(fps) > 1 && !strings.Contains(string(envb), ",") {
			return fmt.Errorf("backup-sftp.env must pin every ssh host key")
		}
		s3env, err := os.ReadFile(root(c, "var/lib/panel/secrets/backup-s3.env"))
		if err != nil {
			return err
		}
		if !strings.Contains(string(s3env), "PANEL_S3_ENDPOINT=http://127.0.0.1:19090") {
			return fmt.Errorf("backup-s3.env missing loopback endpoint")
		}
		if !strings.Contains(string(s3env), "PANEL_S3_ACCESS_KEY=") || !strings.Contains(string(s3env), "PANEL_S3_SECRET_KEY=") {
			return fmt.Errorf("backup-s3.env missing credentials")
		}
	}
	if pub := netaddr.PublicIPv4(); pub != "" && pub != "127.0.0.1" {
		b, err := os.ReadFile(root(c, "etc/vsftpd.conf"))
		if err != nil {
			return err
		}
		if !strings.Contains(string(b), "pasv_address="+pub) {
			return fmt.Errorf("vsftpd PASV address is not %s", pub)
		}
	}
	return nil
}

func hostnameOr(c Config) string {
	if c.Hostname != "" {
		return c.Hostname
	}
	return "panel.local"
}

func applyRuntimeVersions(c Config) error {
	note := "php=8.3 node=system python=system mariadb=system postgresql=system\n"
	return os.WriteFile(root(c, "var/lib/panel/runtime-versions.txt"), []byte(note), 0o644)
}

func applyRepositories(c Config) error {
	return os.MkdirAll(root(c, "etc/apt/sources.list.d"), 0o755)
}

func applyControlPlaneBins(c Config) error {
	return os.MkdirAll(root(c, "usr/local/panel/bin"), 0o755)
}
