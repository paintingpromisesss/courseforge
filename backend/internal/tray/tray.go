package tray

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

// Run starts the system tray icon and blocks the calling goroutine.
// addr is the display address (e.g. "localhost:8080") used for tooltip and browser URL.
// onServerStart is called once the tray is ready — start the HTTP server here.
// onQuit is called when the user selects Quit or a termination signal is received.
func Run(addr string, onServerStart func(), onQuit func()) {
	// Graceful shutdown on OS signals even while tray is active.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		systray.Quit()
	}()

	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("CourseForge")
		systray.SetTooltip("CourseForge — http://" + addr)

		mOpen := systray.AddMenuItem("Open CourseForge", "Open in browser")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop the server and exit")

		onServerStart()

		// Open the browser automatically on launch.
		openBrowser("http://" + addr)

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openBrowser("http://" + addr)
				case <-mQuit.ClickedCh:
					systray.Quit()
				}
			}
		}()
	}, func() {
		// onExit — tray is shutting down.
		onQuit()
	})
}

// Quit asks the tray event loop to stop (safe to call from any goroutine).
func Quit() {
	systray.Quit()
}

func openBrowser(url string) {
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
