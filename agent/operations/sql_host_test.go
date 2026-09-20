package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPostgresConfigWritesListenAndAuth(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cluster := filepath.Join(h.Root, "etc/postgresql/16/main")
	if err := os.MkdirAll(filepath.Join(cluster, "conf.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	hba := "local   all             postgres                                peer\n" +
		"local   all             all                                     peer\n" +
		"host    all             all             127.0.0.1/32            trust\n" +
		"host    all             all             ::1/128                 trust\n"
	if err := os.WriteFile(filepath.Join(cluster, "pg_hba.conf"), []byte(hba), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := h.applyHostConfig(HostConfigSpec{
		WritePostgres:  true,
		PostgresListen: "127.0.0.1",
		PostgresAuth:   "scram-sha-256",
	})
	if err != nil || !res.OK {
		t.Fatalf("apply postgres: %+v %v", res, err)
	}
	if !strings.Contains(res.Message, "postgres") {
		t.Fatalf("applied list should include postgres: %s", res.Message)
	}

	conf, err := os.ReadFile(filepath.Join(cluster, "conf.d/kelmor.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conf), "listen_addresses = '127.0.0.1'") {
		t.Fatalf("listen file: %s", conf)
	}

	updated, err := os.ReadFile(filepath.Join(cluster, "pg_hba.conf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(updated)
	if !strings.Contains(body, "local   all             postgres                                peer") {
		t.Fatalf("unix peer must stay for the control plane: %s", body)
	}
	if !strings.Contains(body, "127.0.0.1/32") || !strings.Contains(body, "scram-sha-256") {
		t.Fatalf("tcp auth method missing: %s", body)
	}
	if strings.Contains(body, "127.0.0.1/32            trust") {
		t.Fatalf("old tcp trust line must be replaced: %s", body)
	}
}

func TestApplyPostgresConfigFailsWhenMissing(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.applyHostConfig(HostConfigSpec{
		WritePostgres:  true,
		PostgresListen: "127.0.0.1",
		PostgresAuth:   "md5",
	})
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("missing postgres must fail honestly: %v", err)
	}
}

func TestApplyPostgresConfigRejectsUnknownAuth(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	cluster := filepath.Join(h.Root, "etc/postgresql/16/main")
	if err := os.MkdirAll(filepath.Join(cluster, "conf.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cluster, "pg_hba.conf"), []byte("local all all peer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := h.applyHostConfig(HostConfigSpec{
		WritePostgres:  true,
		PostgresListen: "127.0.0.1",
		PostgresAuth:   "trust",
	})
	if err == nil {
		t.Fatal("trust must be rejected")
	}
}

func TestProbePostgresReportsMissing(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	status := h.probePostgres()
	if status.Installed {
		t.Fatalf("empty sandbox must not invent postgres: %+v", status)
	}
	if !strings.Contains(status.Message, "not installed") {
		t.Fatalf("honest message: %+v", status)
	}
}

func TestUpgradeMySQLFailsWhenMissing(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	_, err := h.upgradeMySQL("10.11")
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("missing engine must fail honestly: %v", err)
	}
}

func TestUpgradeMySQLStagesWhenMariaDBPresent(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	bin := filepath.Join(h.Root, "usr/bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "mariadb-upgrade"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(h.Root, "usr/sbin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "usr/sbin/mariadbd"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}

	status := h.probeMySQL()
	if !status.Installed || status.Engine != "mariadb" {
		t.Fatalf("probe: %+v", status)
	}

	res, err := h.upgradeMySQL("10.11")
	if err != nil || res.ObservedState != "staged" {
		t.Fatalf("sandbox upgrade: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(h.Root, "var/lib/panel/mysql-upgrade.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeMySQLRejectsUnknownTarget(t *testing.T) {
	h := &Host{Root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(h.Root, "usr/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Root, "usr/bin/mariadb-upgrade"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := h.upgradeMySQL("8.0; rm -rf /")
	if err == nil {
		t.Fatal("hostile target must be rejected")
	}
}
