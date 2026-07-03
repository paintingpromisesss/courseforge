package runner

import (
	"context"
	"os/exec"
	"testing"
)

func requirePostgres(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("initdb"); err != nil {
		t.Skip("initdb not installed — skipping postgres runner test")
	}
}

func TestRun_Postgres_SchemaIsolation(t *testing.T) {
	requirePostgres(t)

	r := New()
	r.AddDriver("postgres", LangDriver{
		RunCmd:      []string{"psql", "-v", "ON_ERROR_STOP=1", "-f", "{file}"},
		Ext:         ".sql",
		NeedsSchema: true,
	})

	// Not configured yet: must fail clearly instead of connecting nowhere.
	_, err := r.Run(context.Background(), RunRequest{Language: "postgres", Code: "SELECT 1;"})
	if err == nil {
		t.Fatal("expected error when postgres is not configured")
	}
}
