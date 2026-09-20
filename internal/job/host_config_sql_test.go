package job

import (
	"testing"

	"github.com/hosting-panel/panel/internal/hostconfig"
)

func TestHostConfigSpecMapsPostgresListenAndAuth(t *testing.T) {
	spec := hostConfigSpec(hostconfig.File{Values: map[string]map[string]string{
		"postgres": {"listen": "0.0.0.0", "auth": "md5"},
	}})
	if !spec.WritePostgres {
		t.Fatal("postgres settings must mark WritePostgres")
	}
	if spec.PostgresListen != "0.0.0.0" || spec.PostgresAuth != "md5" {
		t.Fatalf("postgres spec: %+v", spec)
	}
}
