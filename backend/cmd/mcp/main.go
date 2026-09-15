package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func main() {
	transport := flag.String("transport", "stdio", "transport mode: stdio or sse")
	host := flag.String("host", "127.0.0.1", "host to bind for SSE")
	port := flag.Int("port", 8085, "port to listen on for SSE")
	coursesDir := flag.String("courses-dir", "./courses", "directory with course manifests and files")
	dataDir := flag.String("data-dir", "./data", "directory for application state")
	dbPath := flag.String("db-path", "", "path to submissions sqlite db")
	stateFile := flag.String("state-file", "", "path to active task session json file")
	flag.Parse()

	_ = os.MkdirAll(*dataDir, 0755)
	debugLogPath := filepath.Join(*dataDir, "mcp_debug.log")
	debugFile, _ := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if debugFile != nil {
		defer debugFile.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, debugFile))
		cwd, _ := os.Getwd()
		fmt.Fprintf(debugFile, "[%s] Starting courseforge-mcp (args: %v, pid: %d, cwd: %s)\n", time.Now().Format(time.RFC3339), os.Args, os.Getpid(), cwd)
	} else {
		log.SetOutput(os.Stderr)
	}

	if *dbPath == "" {
		*dbPath = config.DefaultDBPath(*dataDir)
	}
	if *stateFile == "" {
		*stateFile = filepath.Join(*dataDir, "mcp_session.json")
	}

	for _, dir := range []string{*dataDir, *coursesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	db, err := repo.NewDB(*dbPath)
	if err != nil {
		log.Fatalf("failed to open submissions database: %v", err)
	}
	defer db.Close()

	subRepo := repo.NewSubmissionRepository(db)
	progRepo := repo.NewFileProgressRepository(*coursesDir)

	r := runner.New()
	runnersJSON := config.DefaultRunnersJSON(*dataDir)
	if err := r.UseFile(runnersJSON); err != nil {
		log.Printf("warning: failed to load runners.json: %v", err)
	}

	provider, err := mcp.NewCourseForgeProvider(*coursesDir, progRepo, subRepo, r)
	if err != nil {
		log.Fatalf("failed to initialize CourseForge provider: %v", err)
	}

	session, err := mcp.NewFileSessionManager(*stateFile)
	if err != nil {
		log.Fatalf("failed to initialize session manager: %v", err)
	}

	srv, err := mcp.NewServer(mcp.Config{
		Name:        "courseforge-mcp",
		Version:     "1.0.0",
		CoursesDir:  *coursesDir,
		DataDir:     *dataDir,
		DBPath:      *dbPath,
		RunnersJSON: runnersJSON,
	}, provider, session)
	if err != nil {
		log.Fatalf("failed to create MCP server: %v", err)
	}


	switch *transport {
	case "stdio":
		var in io.Reader = os.Stdin
		var out io.Writer = os.Stdout

		if debugFile != nil {
			in = io.TeeReader(os.Stdin, debugFile)
			out = io.MultiWriter(os.Stdout, debugFile)
		}

		if err := srv.ServeStdioWithIO(context.Background(), in, out); err != nil {
			log.Fatalf("MCP stdio server terminated with error: %v", err)
		}
	case "sse":
		addr := *host + ":" + strconv.Itoa(*port)
		fmt.Fprintf(os.Stderr, "CourseForge MCP SSE Server listening on http://%s\n", addr)
		if err := srv.StartSSE(addr); err != nil {
			log.Fatalf("MCP SSE server error: %v", err)
		}
	default:
		log.Fatalf("unsupported transport %q: choose 'stdio' or 'sse'", *transport)
	}
}
