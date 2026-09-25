package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/tray"
)

func runOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	host := fs.String("host", config.DefaultHost, "host the server binds to")
	port := fs.Int("port", config.DefaultPortFromEnv(), "port the server listens on")
	coursesDir := fs.String("courses-dir", "./courses", "directory with course files")
	dataDir := fs.String("data-dir", "./data", "directory for app state")
	fs.Parse(args)

	var courseSlug string
	if rest := fs.Args(); len(rest) > 0 {
		courseSlug = rest[0]
	}

	path := "/"
	if courseSlug != "" {
		path = "/courses/" + courseSlug
	}
	dialAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	url := "http://" + dialAddr + path

	if isUp(dialAddr) {
		tray.OpenBrowser(url)
		return
	}

	fmt.Println("courseforge: starting server in the background...")
	if err := startBackground(*host, *port, *coursesDir, *dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "courseforge open: %v\n", err)
		os.Exit(1)
	}

	if !waitUp(dialAddr, 15*time.Second) {
		fmt.Fprintln(os.Stderr, "courseforge open: server did not come up in time, check the log in data-dir")
		os.Exit(1)
	}
	tray.OpenBrowser(url)
}

func isUp(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitUp(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isUp(addr) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// startBackground launches a detached, tray-less "serve" process so it
// outlives this "open" invocation. The caller opens the browser itself once
// the server answers, so the background instance doesn't also auto-open one.
func startBackground(host string, port int, coursesDir, dataDir string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(dataDir, "courseforge.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(exePath, "serve",
		"--host", host,
		"--port", strconv.Itoa(port),
		"--courses-dir", coursesDir,
		"--data-dir", dataDir,
		"--tray=false",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	detach(cmd)
	return cmd.Start()
}
