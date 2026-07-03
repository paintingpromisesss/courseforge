//go:build windows

package runner

import (
	"context"
	"fmt"
)

// PostgresManager is unavailable on Windows: native Postgres there lacks
// the Unix-socket support this design relies on for no-network isolation.
// Start always fails so callers (di.go) log and disable the driver rather
// than crash — the same "toolchain missing" handling used everywhere else.
type PostgresManager struct{}

func NewPostgresManager(dataDir string) *PostgresManager { return &PostgresManager{} }

func (m *PostgresManager) Start(ctx context.Context) error {
	return fmt.Errorf("postgres unavailable on Windows: Unix sockets not supported")
}

func (m *PostgresManager) Stop() error       { return nil }
func (m *PostgresManager) SocketDir() string { return "" }
