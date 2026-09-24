//go:build !windows

package updater

import (
	"os/exec"
	"syscall"
)

func setupDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
