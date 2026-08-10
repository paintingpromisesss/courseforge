//go:build !windows

package runner

import (
	"context"
	"os/exec"
)

// newCommand is a thin wrapper around exec.Command (no-op on non-Windows).
func newCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// newCommandContext is a thin wrapper around exec.CommandContext (no-op on non-Windows).
func newCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
