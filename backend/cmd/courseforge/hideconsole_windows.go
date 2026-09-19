//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	user32                    = syscall.NewLazyDLL("user32.dll")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procShowWindow            = user32.NewProc("ShowWindow")
)

const swHide = 0

// hideConsoleWindowIfOwned hides this process's console window, but only if
// this process is the console's sole owner — meaning it got a fresh console
// because it was double-clicked from Explorer, not launched from an
// already-open terminal. In the latter case that console belongs to the
// user's shell too; hiding it would hide their whole terminal.
func hideConsoleWindowIfOwned() {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n != 1 {
		return
	}
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swHide)
}
