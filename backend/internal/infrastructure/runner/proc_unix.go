//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopCommand(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func waitAfterStop(_ *exec.Cmd, done <-chan error) (error, bool) {
	select {
	case err := <-done:
		return err, true
	case <-time.After(5 * time.Second):
		return nil, false
	}
}
