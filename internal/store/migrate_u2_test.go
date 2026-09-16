package store

import (
	"strings"
	"testing"

	"github.com/hosting-panel/panel/db"
)

func TestU2CorrectiveMigrationsPreserveOwnershipAndJobs(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
	}{
		{
			name: "migrations/000027_accounts_package_fk_and_reseller_fail_closed.sql",
			parts: []string{
				"accounts_package_id_fkey",
				"VALIDATE CONSTRAINT",
				"UPDATE resellers",
				"privilege_mask = ARRAY[]::TEXT[]",
			},
		},
		{
			name: "migrations/000028_website_domain_ownership.sql",
			parts: []string{
				"UNIQUE (account_id, id)",
				"FOREIGN KEY (account_id, domain_id)",
				"VALIDATE CONSTRAINT",
			},
		},
		{
			name: "migrations/000029_certificate_job_remap.sql",
			parts: []string{
				"UPDATE jobs",
				"DELETE FROM certificates",
				"certificates_account_hostname_uidx",
			},
		},
		{
			name: "migrations/000030_job_target_revision.sql",
			parts: []string{
				"target_revision BIGINT",
				"desired_revision BIGINT",
				"VALIDATE CONSTRAINT",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := db.Migrations.ReadFile(test.name)
			if err != nil {
				t.Fatal(err)
			}
			sql := string(body)
			for _, part := range test.parts {
				if !strings.Contains(sql, part) {
					t.Fatalf("%s does not contain %q", test.name, part)
				}
			}
			if strings.Contains(test.name, "certificate") &&
				strings.Index(sql, "DELETE FROM certificates") < strings.Index(sql, "UPDATE jobs") {
				t.Fatal("certificate duplicates are deleted before jobs are remapped")
			}
		})
	}
}
