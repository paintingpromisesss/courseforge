//go:build windows

package runner

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// PostgresManager owns the lifecycle of the single Postgres cluster used by
// the "postgres" language driver on Windows. Native Postgres there has no
// Unix-socket support, so unlike the Unix implementation this listens on
// TCP, restricted to loopback only (listen_addresses=127.0.0.1) with a port
// picked at Start time so it never collides with another instance (e.g. a
// system-wide Postgres service on the default 5432).
type PostgresManager struct {
	dataDir string
	port    int
}

// NewPostgresManager returns a manager for a cluster rooted at dataDir.
// dataDir is created (and initdb'd) on Start if it doesn't exist yet.
func NewPostgresManager(dataDir string) *PostgresManager {
	return &PostgresManager{dataDir: dataDir}
}

// Host returns the TCP host clients should connect through (PGHOST).
// Only meaningful after Start succeeds.
func (m *PostgresManager) Host() string {
	return "127.0.0.1"
}

// Port returns the port clients should connect through (PGPORT), chosen at
// Start time. Only meaningful after Start succeeds.
func (m *PostgresManager) Port() int {
	return m.port
}

// Start initializes the cluster if needed, starts it listening on loopback
// TCP only, ensures the "courseforge" database and pgtap extension exist,
// and sweeps any schemas left behind by a prior crash.
func (m *PostgresManager) Start(ctx context.Context) error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return fmt.Errorf("create postgres data dir: %w", err)
	}

	abs, err := filepath.Abs(m.dataDir)
	if err != nil {
		return fmt.Errorf("resolve postgres data dir: %w", err)
	}
	m.dataDir = abs

	port, err := freeLoopbackPort()
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	m.port = port

	if _, err := os.Stat(filepath.Join(m.dataDir, "PG_VERSION")); os.IsNotExist(err) {
		out, err := exec.CommandContext(ctx, "initdb", "-D", m.dataDir, "--auth=trust", "--no-sync").CombinedOutput()
		if err != nil {
			return fmt.Errorf("initdb: %w: %s", err, out)
		}
	}

	out, err := exec.CommandContext(ctx, "pg_ctl", "-D", m.dataDir, "-w",
		"-o", fmt.Sprintf("-c listen_addresses=127.0.0.1 -c port=%d", m.port),
		"-l", filepath.Join(m.dataDir, "server.log"),
		"start").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl start: %w: %s", err, out)
	}

	if out, err := exec.CommandContext(ctx, "createdb", "-h", m.Host(), "-p", strconv.Itoa(m.port), "courseforge").CombinedOutput(); err != nil &&
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

func (m *PostgresManager) reapOrphanSchemas(ctx context.Context) error {
	return m.psql(ctx, pgtapSweepSQL)
}

func (m *PostgresManager) psql(ctx context.Context, stmt string) error {
	out, err := exec.CommandContext(ctx, "psql", "-h", m.Host(), "-p", strconv.Itoa(m.port), "-d", "courseforge",
		"-v", "ON_ERROR_STOP=1", "-c", stmt).CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql: %w: %s", err, out)
	}
	return nil
}

// freeLoopbackPort asks the OS for an unused TCP port on 127.0.0.1. There's
// an inherent TOCTOU race (the port could be grabbed between Close and
// postgres binding it) — acceptable here since this only runs once at
// startup, not per-request.
func freeLoopbackPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
