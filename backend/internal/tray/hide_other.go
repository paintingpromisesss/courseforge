//go:build !windows

package tray

import "os/exec"

func hideWindow(_ *exec.Cmd) {}
