//go:build !windows

package runner

import (
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
	dataDir string
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
	// Create dataDir if needed.
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return fmt.Errorf("mkdir dataDir: %w", err)
	}

	// Check if cluster already initialized.
	pgVersion := filepath.Join(m.dataDir, "PG_VERSION")
	if _, err := os.Stat(pgVersion); err == nil {
		// Already initialized; start it.
		if err := m.pgCtl(ctx, "start"); err != nil {
			return err
		}
	} else {
		// Not initialized; initdb then start.
		out, err := exec.CommandContext(ctx, "initdb", "-D", m.dataDir, "-U", "postgres").CombinedOutput()
		if err != nil {
			return fmt.Errorf("initdb: %w: %s", err, out)
		}
		if err := m.pgCtl(ctx, "start"); err != nil {
			return err
		}
	}

	// Ensure courseforge database exists.
	if out, err := exec.CommandContext(ctx, "createdb", "-h", m.dataDir, "-U", "postgres", "courseforge").CombinedOutput(); err != nil {
		// Ignore "already exists" error.
		errStr := string(out)
		if errStr != "" && errStr != "createdb: error: database \"courseforge\" already exists\n" {
			return fmt.Errorf("createdb courseforge: %w: %s", err, out)
		}
	}

	// Ensure pgtap extension exists.
	if out, err := exec.CommandContext(ctx, "psql", "-h", m.dataDir, "-d", "courseforge",
		"-tAc", "CREATE EXTENSION IF NOT EXISTS pgtap").CombinedOutput(); err != nil {
		return fmt.Errorf("pgtap install: %w: %s", err, out)
	}

	// Reap any orphaned schemas.
	if err := m.reapOrphanSchemas(ctx); err != nil {
		return fmt.Errorf("reapOrphanSchemas: %w", err)
	}

	return nil
}

// Stop shuts the cluster down cleanly.
func (m *PostgresManager) Stop() error {
	out, err := exec.Command("pg_ctl", "-D", m.dataDir, "stop", "-m", "fast").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl stop: %w: %s", err, out)
	}
	return nil
}

// reapOrphanSchemas drops any run schema left behind by a courseforge
// process that was killed mid-run, so a crash never leaves permanent
// clutter in the courseforge database.
func (m *PostgresManager) reapOrphanSchemas(ctx context.Context) error {
	stmt := `DO $$
DECLARE r record;
BEGIN
  FOR r IN SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'cf\_run\_%' LOOP
    EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(r.schema_name) || ' CASCADE';
  END LOOP;
END $$;`

	return m.psql(ctx, stmt)
}

func (m *PostgresManager) psql(ctx context.Context, stmt string) error {
	out, err := exec.CommandContext(ctx, "psql", "-h", m.dataDir, "-d", "courseforge", "-c", stmt).CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql: %w: %s", err, out)
	}
	return nil
}

func (m *PostgresManager) pgCtl(ctx context.Context, action string) error {
	out, err := exec.CommandContext(ctx, "pg_ctl", "-D", m.dataDir, "-l",
		filepath.Join(m.dataDir, "postgres.log"), "-w", action).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl %s: %w: %s", action, err, out)
	}
	return nil
}
