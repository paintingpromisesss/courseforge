//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func stopCommand(cmd *exec.Cmd) error {
	killTree := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	if err := killTree.Run(); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func waitAfterStop(cmd *exec.Cmd, done <-chan error) (error, bool) {
	select {
	case err := <-done:
		return err, true
	case <-time.After(500 * time.Millisecond):
		_ = cmd.Process.Release()
		return nil, false
	}
}
