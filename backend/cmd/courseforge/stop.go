package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/config"
)

func runStop(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	port := fs.Int("port", config.DefaultPortFromEnv(), "port the server listens on")
	fs.Parse(args)

	addr := net.JoinHostPort(config.DefaultHost, strconv.Itoa(*port))
	if !isUp(addr) {
		fmt.Println("courseforge: not running")
		return
	}

	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/api/shutdown", nil)
	req.Header.Set("X-Courseforge-Stop", "1")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "courseforge stop: %v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(os.Stderr, "courseforge stop: unexpected status %s\n", resp.Status)
		os.Exit(1)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !isUp(addr) {
			fmt.Println("courseforge: stopped")
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "courseforge stop: server did not exit in time")
	os.Exit(1)
}
