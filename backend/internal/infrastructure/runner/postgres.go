//go:build !windows

package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PostgresManager owns the lifecycle of the single Postgres cluster used
// by the "postgres" language driver. One cluster, one database
// ("courseforge") — isolation between runs happens via schemas
// (Runner.createSchema/dropSchema), so the cluster itself never grows a
// new database or process per submission.
type PostgresManager struct {
	dataDir string // cluster data directory; also used as the Unix socket directory
}

// NewPostgresManager returns a manager for a cluster rooted at dataDir.
// dataDir is created (and initdb'd) on Start if it doesn't exist yet.
func NewPostgresManager(dataDir string) *PostgresManager {
	return &PostgresManager{dataDir: dataDir}
}

// SocketDir returns the Unix socket directory clients should connect
// through (PGHOST). Only meaningful after Start succeeds.
func (m *PostgresManager) SocketDir() string {
	return m.dataDir
}

// Start initializes the cluster if needed, starts it listening on a Unix
// socket only (no TCP), ensures the "courseforge" database and pgtap
// extension exist, and sweeps any schemas left behind by a prior crash.
func (m *PostgresManager) Start(ctx context.Context) error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return fmt.Errorf("create postgres data dir: %w", err)
	}

	if _, err := os.Stat(filepath.Join(m.dataDir, "PG_VERSION")); os.IsNotExist(err) {
		out, err := exec.CommandContext(ctx, "initdb", "-D", m.dataDir, "--auth=trust", "--no-sync").CombinedOutput()
		if err != nil {
			return fmt.Errorf("initdb: %w: %s", err, out)
		}
	}

	out, err := exec.CommandContext(ctx, "pg_ctl", "-D", m.dataDir, "-w",
		"-o", fmt.Sprintf("-c listen_addresses='' -c unix_socket_directories=%s", m.dataDir),
		"-l", filepath.Join(m.dataDir, "server.log"),
		"start").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl start: %w: %s", err, out)
	}

	if out, err := exec.CommandContext(ctx, "createdb", "-h", m.dataDir, "courseforge").CombinedOutput(); err != nil &&
		!bytes.Contains(out, []byte("already exists")) {
		return fmt.Errorf("createdb: %w: %s", err, out)
	}

	if err := m.psql(ctx, "CREATE EXTENSION IF NOT EXISTS pgtap"); err != nil {
		return fmt.Errorf("create pgtap extension: %w", err)
	}

	return m.reapOrphanSchemas(ctx)
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
	const sweep = `DO $$
DECLARE r record;
BEGIN
  FOR r IN SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'cf\_run\_%' LOOP
    EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.schema_name) || ' CASCADE';
  END LOOP;
END $$;`
	return m.psql(ctx, sweep)
}

func (m *PostgresManager) psql(ctx context.Context, stmt string) error {
	out, err := exec.CommandContext(ctx, "psql", "-h", m.dataDir, "-d", "courseforge",
		"-v", "ON_ERROR_STOP=1", "-c", stmt).CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql: %w: %s", err, out)
	}
	return nil
}
