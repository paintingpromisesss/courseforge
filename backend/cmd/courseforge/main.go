package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/di"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "mcp" || os.Args[1] == "--mcp") {
		mcp.RunCLI(os.Args[2:])
		return
	}

	host := flag.String("host", "127.0.0.1", "host to bind")
	port := flag.Int("port", 8080, "port to listen on")
	coursesDir := flag.String("courses-dir", config.DefaultCoursesDir(), "directory with course files")
	dataDir := flag.String("data-dir", config.DefaultDataDir(), "directory for app state")
	dbPath := flag.String("db-path", "", "path to submissions sqlite db")
	frontendDir := flag.String("frontend-dir", defaultFrontendDir(), "directory with built frontend assets")
	enableTray := flag.Bool("tray", true, "show system tray icon")
	flag.Parse()

	resolvedCourses := *coursesDir
	if resolvedCourses == "" || resolvedCourses == "./courses" {
		if fi, err := os.Stat(resolvedCourses); err != nil || !fi.IsDir() {
			resolvedCourses = config.DefaultCoursesDir()
		}
	}
	resolvedData := *dataDir
	if resolvedData == "" || resolvedData == "./data" {
		if fi, err := os.Stat(resolvedData); err != nil || !fi.IsDir() {
			resolvedData = config.DefaultDataDir()
		}
	}

	cfg := &config.Config{
		DataDir:     resolvedData,
		CoursesDir:  resolvedCourses,
		RunnersJSON: config.DefaultRunnersJSON(resolvedData),
		FrontendDir: *frontendDir,
		Addr:        *host + ":" + strconv.Itoa(*port),
		DBPath:      *dbPath,
		EnableTray:  *enableTray,
	}
	if cfg.DBPath == "" {
		cfg.DBPath = config.DefaultDBPath(resolvedData)
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

