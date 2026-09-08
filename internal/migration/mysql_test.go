package migration

import "testing"

func TestExtractMySQLDatabase(t *testing.T) {
	dump := []byte(`CREATE DATABASE IF NOT EXISTS ` + "`acme42_wp`" + `;
USE ` + "`acme42_wp`" + `;
CREATE TABLE wp_options (option_name varchar(64));
INSERT INTO wp_options VALUES ('blogname');
CREATE DATABASE ` + "`acme42_store`" + `;
USE ` + "`acme42_store`" + `;
CREATE TABLE products (id int);
`)
	got := string(ExtractMySQLDatabase(dump, "acme42_wp"))
	if got != "CREATE TABLE wp_options (option_name varchar(64));\nINSERT INTO wp_options VALUES ('blogname');\n" {
		t.Fatalf("%q", got)
	}
	store := string(ExtractMySQLDatabase(dump, "acme42_store"))
	if store != "CREATE TABLE products (id int);\n" {
		t.Fatalf("%q", store)
	}
	if ExtractMySQLDatabase(dump, "missing") != nil {
		t.Fatal("expected missing extract to be empty")
	}
	plain := []byte("CREATE TABLE only (id int);\n")
	if string(ExtractMySQLDatabase(plain, "any")) != string(plain) {
		t.Fatal("single-db dump should pass through")
	}
}
