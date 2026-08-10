//go:build windows

package runner

import (
	"context"
	"os/exec"
	"syscall"
)

// newCommand is like exec.Command but sets CREATE_NO_WINDOW so
// child processes don't flash a console when the parent is a GUI app.
func newCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
	return cmd
}

// newCommandContext is like exec.CommandContext but sets CREATE_NO_WINDOW.
func newCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
	return cmd
}
