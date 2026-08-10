//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const createNoWindow = 0x08000000

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

// HideWindow sets CREATE_NO_WINDOW on the command so no console
// window flashes when the parent process is a GUI application.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

func stopCommand(cmd *exec.Cmd) error {
	killTree := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	HideWindow(killTree)
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

