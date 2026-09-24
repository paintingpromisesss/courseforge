package mcp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// RunCLI parses flags from args and executes the MCP server.
// If MCP is disabled in CourseForge settings (mcp_server_config.json),
// it exits with an error unless -force is specified.
func RunCLI(args []string) {
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = args[1:]
	}

	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	transport := fs.String("transport", "stdio", "transport mode: stdio or sse")
	host := fs.String("host", "127.0.0.1", "host to bind for standalone SSE")
	port := fs.Int("port", 8085, "port to listen on for standalone SSE")
	coursesDir := fs.String("courses-dir", "", "directory with course manifests and files")
	dataDir := fs.String("data-dir", "", "directory for application state")
	dbPath := fs.String("db-path", "", "path to submissions sqlite db")
	stateFile := fs.String("state-file", "", "path to active task session json file")
	serverURL := fs.String("server-url", "http://127.0.0.1:8080", "CourseForge server URL to discover and ping")
	force := fs.Bool("force", false, "bypass disabled check in settings")

	_ = fs.Parse(args)

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	autoDiscover := !set["courses-dir"] || !set["data-dir"]

	if autoDiscover {
		c, d, ok := DiscoverServerDirs(*serverURL)
		if ok {
			*coursesDir, *dataDir = c, d
			log.Printf("discovered running CourseForge server at %s (courses: %s, data: %s)", *serverURL, c, d)
		} else {
			log.Printf("CourseForge server is not running on %s. Starting MCP in standby auto-discovery mode.", *serverURL)
		}
	} else {
		if *coursesDir == "" || *coursesDir == "./courses" {
			if fi, err := os.Stat(*coursesDir); err != nil || !fi.IsDir() {
				*coursesDir = config.DefaultCoursesDir()
			}
		}
		if *dataDir == "" || *dataDir == "./data" {
			if fi, err := os.Stat(*dataDir); err != nil || !fi.IsDir() {
				*dataDir = config.DefaultDataDir()
			}
		}
	}

	logDir := *dataDir
	if logDir == "" {
		logDir = config.DefaultDataDir()
	}
	_ = os.MkdirAll(logDir, 0755)
	debugLogPath := filepath.Join(logDir, "mcp_debug.log")
	debugFile, _ := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if debugFile != nil {
		defer debugFile.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, debugFile))
		cwd, _ := os.Getwd()
		fmt.Fprintf(debugFile, "[%s] Starting CourseForge MCP (args: %v, pid: %d, cwd: %s)\n", time.Now().Format(time.RFC3339), args, os.Getpid(), cwd)
	} else {
		log.SetOutput(os.Stderr)
	}

	var provider Provider
	var session SessionManager
	var dbCloser io.Closer

	if *coursesDir != "" && *dataDir != "" {
		// Check if MCP is disabled in settings
		mcpRepo := repo.NewMCPConfigRepository(*dataDir)
		if mcpCfg, err := mcpRepo.Get(context.Background()); err == nil && mcpCfg != nil {
			if !mcpCfg.Enabled && !*force {
				if !autoDiscover {
					fmt.Fprintf(os.Stderr, "CourseForge MCP server is disabled in settings. Enable it in CourseForge Settings -> MCP-сервер.\n")
					os.Exit(1)
				}
				log.Printf("CourseForge MCP server is disabled in settings. Starting in standby mode.")
				*coursesDir = ""
				*dataDir = ""
			}
		}

		if *coursesDir != "" && *dataDir != "" {
			for _, dir := range []string{*dataDir, *coursesDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					log.Fatalf("failed to create directory %s: %v", dir, err)
				}
			}

			resolvedDBPath := *dbPath
			if resolvedDBPath == "" {
				resolvedDBPath = config.DefaultDBPath(*dataDir)
			}
			resolvedStateFile := *stateFile
			if resolvedStateFile == "" {
				resolvedStateFile = filepath.Join(*dataDir, "mcp_session.json")
			}

			db, err := repo.NewDB(resolvedDBPath)
			if err != nil {
				log.Fatalf("failed to open submissions database: %v", err)
			}
			dbCloser = db

			subRepo := repo.NewSubmissionRepository(db)
			progRepo := repo.NewFileProgressRepository(*coursesDir)

			r := runner.New()
			runnersJSON := config.DefaultRunnersJSON(*dataDir)
			if err := r.UseFile(runnersJSON); err != nil {
				log.Printf("warning: failed to load runners.json: %v", err)
			}

			prov, err := NewCourseForgeProvider(*coursesDir, *dataDir, progRepo, subRepo, r)
			if err != nil {
				log.Fatalf("failed to initialize CourseForge provider: %v", err)
			}
			provider = prov

			sess, err := NewFileSessionManager(resolvedStateFile)
			if err != nil {
				log.Fatalf("failed to initialize session manager: %v", err)
			}
			session = sess
		}
	}

	srv, err := NewServer(Config{
		Name:         "courseforge-mcp",
		Version:      "1.0.0",
		CoursesDir:   *coursesDir,
		DataDir:      *dataDir,
		DBPath:       *dbPath,
		ServerURL:    *serverURL,
		AutoDiscover: autoDiscover,
		Force:        *force,
	}, provider, session)
	if err != nil {
		if dbCloser != nil {
			_ = dbCloser.Close()
		}
		log.Fatalf("failed to create MCP server: %v", err)
	}
	defer srv.Close()
	if dbCloser != nil {
		defer dbCloser.Close()
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
