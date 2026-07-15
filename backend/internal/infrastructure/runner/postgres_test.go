package runner

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

	dir := t.TempDir()
	m := NewPostgresManager(filepath.Join(dir, "pg"))
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	r := New()
	r.ConfigurePostgres(m.Host(), m.Port())

	res, err := r.Run(context.Background(), RunRequest{
		Language: "postgres",
		Code:     "CREATE TABLE t (id int); INSERT INTO t VALUES (1);",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("first run exit %d stderr: %s", res.ExitCode, res.Stderr)
	}

	// A second, independent run must not see the first run's table — proves
	// schema-per-run isolation, not just "it connected".
	res2, err := r.Run(context.Background(), RunRequest{
		Language: "postgres",
		Code:     "SELECT * FROM t;",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2.ExitCode == 0 {
		t.Fatalf("expected second run to fail (table t must not leak across schemas), got exit 0: %s", res2.Stdout)
	}
}

func TestPostgresManager_StartCreatesDatabaseAndExtension(t *testing.T) {
	requirePostgres(t)

	dir := t.TempDir()
	m := NewPostgresManager(filepath.Join(dir, "pg"))
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	out, err := exec.Command("psql", "-h", m.Host(), "-p", strconv.Itoa(m.Port()), "-d", "courseforge",
		"-tAc", "SELECT extname FROM pg_extension WHERE extname = 'pgtap'").CombinedOutput()
	if err != nil {
		t.Fatalf("psql check: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "pgtap" {
		t.Fatalf("expected pgtap extension installed, got: %s", out)
	}
}

func TestPostgresManager_ReapsOrphanSchemas(t *testing.T) {
	requirePostgres(t)

	dir := t.TempDir()
	m := NewPostgresManager(filepath.Join(dir, "pg"))
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	// Simulate a schema orphaned by a crashed run.
	if out, err := exec.Command("psql", "-h", m.Host(), "-p", strconv.Itoa(m.Port()), "-d", "courseforge",
		"-c", "CREATE SCHEMA cf_run_deadbeef").CombinedOutput(); err != nil {
		t.Fatalf("create orphan schema: %v: %s", err, out)
	}

	if err := m.reapOrphanSchemas(context.Background()); err != nil {
		t.Fatalf("reapOrphanSchemas: %v", err)
	}

	out, err := exec.Command("psql", "-h", m.Host(), "-p", strconv.Itoa(m.Port()), "-d", "courseforge",
		"-tAc", "SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'cf_run_deadbeef'").CombinedOutput()
	if err != nil {
		t.Fatalf("psql check: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "0" {
		t.Fatalf("expected orphan schema reaped, still present: %s", out)
	}
}
