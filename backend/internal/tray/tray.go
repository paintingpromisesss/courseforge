package tray

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/gogpu/systray"
)

//go:embed icon.png
var iconData []byte

var (
	currentTray *systray.SystemTray
	trayMu      sync.Mutex
)

// Run starts the system tray icon and blocks the calling goroutine.
// addr is the display address (e.g. "localhost:6770") used for tooltip and browser URL.
// onServerStart is called once the tray is ready — start the HTTP server here.
// onQuit is called when the user selects Quit or a termination signal is received.
func Run(addr string, onServerStart func(), onQuit func()) {
	t := systray.New()
	trayMu.Lock()
	currentTray = t
	trayMu.Unlock()

	menu := systray.NewMenu()
	menu.Add("Open CourseForge", func() {
		OpenBrowser("http://" + addr)
	})
	menu.AddSeparator()
	menu.Add("Quit", func() {
		Quit()
	})

	t.SetIcon(iconData).
		SetTooltip("CourseForge — http://" + addr).
		SetMenu(menu)

	t.OnClick(func() {
		OpenBrowser("http://" + addr)
	})

	t.Show()

	// Graceful shutdown on OS signals even while tray is active.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		Quit()
	}()

	if onServerStart != nil {
		onServerStart()
	}

	// Open the browser automatically on launch unless suppressed.
	if os.Getenv("COURSEFORGE_NO_OPEN") == "" {
		OpenBrowser("http://" + addr)
	}

	if err := t.Run(); err != nil {
		log.Printf("tray error: %v", err)
	}

	if onQuit != nil {
		onQuit()
	}
}

// Quit asks the tray event loop to stop (safe to call from any goroutine).
func Quit() {
	trayMu.Lock()
	t := currentTray
	currentTray = nil
	trayMu.Unlock()

	if t != nil {
		t.Remove()
	}
}

// OpenBrowser opens url in the OS default browser.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		hideWindow(cmd)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("failed to open browser: %v", err)
	}
}
