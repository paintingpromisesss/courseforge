//go:build windows

package main

import (
	"os"
	"syscall"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
)

const attachParentProcess = ^uintptr(0) // (DWORD)-1 = 0xFFFFFFFF

// attachConsoleIfAvailable attaches to the parent console if this GUI-subsystem
// executable was launched from an existing terminal (CMD, PowerShell, terminal).
// If launched by double-clicking in Explorer, AttachConsole returns false,
// leaving the process with zero console window (clean GUI/tray launch).
func attachConsoleIfAvailable() {
	outH, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	fileType, _ := syscall.GetFileType(outH)
	isPiped := fileType == syscall.FILE_TYPE_PIPE || fileType == syscall.FILE_TYPE_DISK

	r, _, _ := procAttachConsole.Call(attachParentProcess)
	if r == 0 {
		return
	}

	// If stdout was not explicitly piped/redirected, bind it to CONOUT$ so terminal output works
	if !isPiped {
		if f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
			os.Stdout = f
			os.Stderr = f
		}
		if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
			os.Stdin = f
		}
	}
}
