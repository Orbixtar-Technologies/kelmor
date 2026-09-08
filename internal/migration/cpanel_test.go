package migration

import (
	"path/filepath"
	"testing"
)

func TestFromCPanel(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "cpanel-acme42")
	exp, err := FromCPanel(root, "acme42")
	if err != nil {
		t.Fatal(err)
	}
	if exp.Account.PrimaryDomain != "acme.test" {
		t.Fatal(exp.Account)
	}
	if len(exp.Databases) != 2 {
		t.Fatalf("dbs %d", len(exp.Databases))
	}
	if len(exp.Records) == 0 {
		t.Fatal("expected dns records")
	}
	found := false
	for _, m := range exp.Mailboxes {
		if m.LocalPart == "info" {
			found = true
		}
	}
	if !found {
		t.Fatal(exp.Mailboxes)
	}
	if exp.Homedir == "" {
		t.Fatal("expected homedir path from extracted cpmove tree")
	}
}
