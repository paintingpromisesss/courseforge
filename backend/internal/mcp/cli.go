package mcp

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"


	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// discoverServerDirs asks the local CourseForge server (GET /api/info) for its dirs.
func discoverServerDirs() (coursesDir, dataDir string, ok bool) {
	client := http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:8080/api/info")
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	var info struct {
		CoursesDir string `json:"courses_dir"`
		DataDir    string `json:"data_dir"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&info) != nil {
		return "", "", false
	}
	return info.CoursesDir, info.DataDir, info.CoursesDir != "" && info.DataDir != ""
}

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
	coursesDir := fs.String("courses-dir", config.DefaultCoursesDir(), "directory with course manifests and files")
	dataDir := fs.String("data-dir", config.DefaultDataDir(), "directory for application state")
	dbPath := fs.String("db-path", "", "path to submissions sqlite db")
	stateFile := fs.String("state-file", "", "path to active task session json file")
	force := fs.Bool("force", false, "bypass disabled check in settings")

	_ = fs.Parse(args)

	// Without explicit dirs, adopt the ones of a running server so both share courses and data.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["courses-dir"] && !set["data-dir"] {
		c, d, ok := discoverServerDirs()
		if !ok {
			fmt.Fprintln(os.Stderr, "CourseForge server is not running on 127.0.0.1:8080. Start it (courseforge serve) or pass --courses-dir and --data-dir.")
			os.Exit(1)
		}
		*coursesDir, *dataDir = c, d
	}

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


	// Check if MCP is disabled in settings
	mcpRepo := repo.NewMCPConfigRepository(*dataDir)
	if mcpCfg, err := mcpRepo.Get(context.Background()); err == nil && mcpCfg != nil {
		if !mcpCfg.Enabled && !*force {
			fmt.Fprintf(os.Stderr, "CourseForge MCP server is disabled in settings. Enable it in CourseForge Settings -> MCP-сервер.\n")
			os.Exit(1)
		}
	}

	_ = os.MkdirAll(*dataDir, 0755)
	debugLogPath := filepath.Join(*dataDir, "mcp_debug.log")
	debugFile, _ := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if debugFile != nil {
		defer debugFile.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, debugFile))
		cwd, _ := os.Getwd()
		fmt.Fprintf(debugFile, "[%s] Starting CourseForge MCP (args: %v, pid: %d, cwd: %s)\n", time.Now().Format(time.RFC3339), args, os.Getpid(), cwd)
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

	provider, err := NewCourseForgeProvider(*coursesDir, progRepo, subRepo, r)
	if err != nil {
		log.Fatalf("failed to initialize CourseForge provider: %v", err)
	}

	session, err := NewFileSessionManager(*stateFile)
	if err != nil {
		log.Fatalf("failed to initialize session manager: %v", err)
	}

	srv, err := NewServer(Config{
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
