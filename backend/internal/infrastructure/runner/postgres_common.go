package runner

import (
	"context"
	"fmt"
	"os/exec"
)

// NewPostgresManager returns a manager for a cluster rooted at dataDir.
// dataDir is created (and initdb'd) on Start if it doesn't exist yet.
func NewPostgresManager(dataDir string) *PostgresManager {
	return &PostgresManager{dataDir: dataDir}
}

// Stop shuts the cluster down cleanly.
func (m *PostgresManager) Stop() error {
	out, err := exec.Command("pg_ctl", "-D", m.dataDir, "-w", "-m", "fast", "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl stop: %w: %s", err, out)
	}
	return nil
}

// reapOrphanSchemas drops any run schema left behind by a courseforge
// process that was killed mid-run, so a crash never leaves permanent
// clutter in the courseforge database.
func (m *PostgresManager) reapOrphanSchemas(ctx context.Context) error {
	return m.psql(ctx, pgtapSweepSQL)
}

func (m *PostgresManager) psql(ctx context.Context, stmt string) error {
	args := append(m.connArgs(), "-d", "courseforge", "-v", "ON_ERROR_STOP=1", "-c", stmt)
	out, err := exec.CommandContext(ctx, "psql", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql: %w: %s", err, out)
	}
	return nil
}

// pgtapSweepSQL drops any run schema left behind by a courseforge process
// that was killed mid-run, so a crash never leaves permanent clutter in the
// courseforge database. Shared between the Unix and Windows PostgresManager
// implementations.
const pgtapSweepSQL = `DO $$
DECLARE r record;
BEGIN
  FOR r IN SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'cf\_run\_%' LOOP
    EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.schema_name) || ' CASCADE';
  END LOOP;
END $$;`
