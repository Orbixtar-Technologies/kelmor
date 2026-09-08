package migration

import (
	"strings"
)

// ExtractMySQLDatabase returns the statements that belong to name from a
// multi-database mysqldump, without CREATE DATABASE / USE wrappers so the
// restore can target a remapped destination.
func ExtractMySQLDatabase(dump []byte, name string) []byte {
	if name == "" || len(dump) == 0 {
		return nil
	}
	lines := strings.Split(string(dump), "\n")
	var out []string
	capturing := false
	found := false
	anyCreate := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		upper := strings.ToUpper(trim)
		if strings.HasPrefix(upper, "CREATE DATABASE") {
			anyCreate = true
			capturing = extractIdent(line) == name
			if capturing {
				found = true
			}
			continue
		}
		if capturing && strings.HasPrefix(upper, "USE ") {
			continue
		}
		if capturing {
			out = append(out, line)
		}
	}
	if found {
		body := strings.TrimSpace(strings.Join(out, "\n"))
		if body == "" {
			return []byte("")
		}
		return []byte(body + "\n")
	}
	if !anyCreate {
		return dump
	}
	return nil
}
