package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/paintingpromisesss/courseforge/internal/di"
)

func main() {
	host := flag.String("host", "127.0.0.1", "host to bind")
	port := flag.Int("port", 8080, "port to listen on")
	coursesDir := flag.String("courses-dir", "./courses", "directory with course files")
	dataDir := flag.String("data-dir", "./data", "directory for app state")
	dbPath := flag.String("db-path", "", "path to submissions sqlite db")
	frontendDir := flag.String("frontend-dir", defaultFrontendDir(), "directory with built frontend assets")
	flag.Parse()

	cfg := &di.Config{
		DataDir:     *dataDir,
		CoursesDir:  *coursesDir,
		RunnersDir:  filepath.Join(*dataDir, "runners", "bin"),
		RunnersJSON: filepath.Join(*dataDir, "runners.json"),
		FrontendDir: *frontendDir,
		Addr:        *host + ":" + strconv.Itoa(*port),
		DBPath:      *dbPath,
	}
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(*dataDir, "courseforge.db")
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
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}

	return "./frontend/dist"
}
