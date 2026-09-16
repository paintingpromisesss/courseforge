//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// detach configures cmd so it survives after this process exits: no console
// window, and not part of this process's group (so a parent's Ctrl+C or
// terminal close doesn't take it down too).
func detach(cmd *exec.Cmd) {
	const detachedProcess = 0x00000008
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}
}
