package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/di"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		runServe(args)
		return
	}

	switch args[0] {
	case "serve":
		runServe(args[1:])
	case "mcp", "--mcp":
		mcp.RunCLI(args[1:])
	case "doctor":
		runDoctor(args[1:])
	case "open":
		runOpen(args[1:])
	case "update":
		runUpdate(args[1:])
	case "version", "-v", "--version":
		fmt.Printf("courseforge %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		printHelp()
	default:
		if strings.HasPrefix(args[0], "-") {
			// Backward compat: `courseforge --port 9000` (no subcommand) still serves.
			runServe(args)
			return
		}
		fmt.Fprintf(os.Stderr, "courseforge: unknown command %q\n\n", args[0])
		printHelp()
		os.Exit(2)
	}
}

func printHelp() {
	fmt.Print(`courseforge — self-hosted coding-course platform

Usage:
  courseforge [flags]              start the server (default command)
  courseforge serve [flags]        start the server
  courseforge open [course-slug]   open the app in your browser, starting the server if needed
  courseforge doctor               check which language toolchains are usable
  courseforge update [--tag vX.Y.Z] update to the latest (or a specific) release
  courseforge mcp [flags]          run the MCP server
  courseforge version              print the version
  courseforge help                 show this help

Run "courseforge <command> -h" for flags on a specific command.
`)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	host := fs.String("host", "127.0.0.1", "host to bind")
	port := fs.Int("port", 8080, "port to listen on")
	coursesDir := fs.String("courses-dir", "./courses", "directory with course files")
	dataDir := fs.String("data-dir", "./data", "directory for app state")
	dbPath := fs.String("db-path", "", "path to submissions sqlite db")
	frontendDir := fs.String("frontend-dir", defaultFrontendDir(), "directory with built frontend assets")
	enableTray := fs.Bool("tray", true, "show system tray icon")
	fs.Parse(args)

	cfg := &config.Config{
		DataDir:     *dataDir,
		CoursesDir:  *coursesDir,
		RunnersJSON: config.DefaultRunnersJSON(*dataDir),
		FrontendDir: *frontendDir,
		Addr:        *host + ":" + strconv.Itoa(*port),
		DBPath:      *dbPath,
		EnableTray:  *enableTray,
	}
	if cfg.EnableTray {
		hideConsoleWindowIfOwned()
	}
	if cfg.DBPath == "" {
		cfg.DBPath = config.DefaultDBPath(*dataDir)
	}

	if err := di.Run(cfg); err != nil {
		log.Fatal(err)
	}
}

func defaultFrontendDir() string {
	candidates := []string{
		"./frontend/dist",
		"../frontend/dist",
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "frontend", "dist"),
			filepath.Join(exeDir, "..", "frontend", "dist"),
			filepath.Join(exeDir, "frontend-dist"),
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}

	return ""
}
