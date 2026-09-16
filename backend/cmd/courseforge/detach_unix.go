//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detach puts cmd in its own session so it survives after this process
// exits, instead of dying with it or receiving its signals.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
