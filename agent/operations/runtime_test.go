package operations

import "testing"

func TestIdent(t *testing.T) {
	if !ident("livehost_shop") || !ident("livehost_u") {
		t.Fatal("valid identifiers rejected")
	}
	if ident("1bad") || ident("bad-name") || ident("") {
		t.Fatal("invalid identifiers accepted")
	}
}

func TestCreateHostedDatabaseRejectsIdent(t *testing.T) {
	h := &Host{}
	if _, err := h.createHostedDatabase("mariadb", "bad-name", "okuser", "pw"); err == nil {
		t.Fatal("expected invalid identifier")
	}
	res, err := h.createHostedDatabase("mariadb", "okdb", "okuser", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatal(res)
	}
}

func TestRunFixedRejectsSQLSemicolon(t *testing.T) {
	if _, err := runFixed("/usr/bin/mariadb", "-e", "SELECT 1; SELECT 2"); err == nil {
		t.Fatal("semicolon batch must be rejected — issue one statement per call")
	}
	if _, err := runFixed("/usr/bin/mariadb", "-e", "CREATE DATABASE `x`"); err == nil {
		t.Fatal("backticks must be rejected — use validated unquoted identifiers")
	}
}
