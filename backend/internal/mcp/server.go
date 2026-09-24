package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// Server wraps the mark3labs MCP server with CourseForge specific dependencies.
type Server struct {
	mu                sync.RWMutex
	mcpServer         *server.MCPServer
	provider          Provider
	session           SessionManager
	logger            Logger
	cfg               Config
	lastPing          time.Time
	lastServerOnline  bool
	currentCoursesDir string
	currentDataDir    string
	dbCloser          io.Closer
}

// Logger abstracts logging for the MCP server.
type Logger interface {
	Printf(format string, v ...any)
}

type defaultLogger struct{}

func (defaultLogger) Printf(format string, v ...any) {}

// Config holds configuration for the MCP Server.
type Config struct {
	Name         string
	Version      string
	CoursesDir   string
	DataDir      string
	DBPath       string
	RunnersJSON  string
	ServerURL    string
	AutoDiscover bool
	Force        bool
	Logger       Logger
}

// NewServer creates and initializes a new CourseForge MCP Server instance.
func NewServer(cfg Config, provider Provider, session SessionManager) (*Server, error) {
	if cfg.Name == "" {
		cfg.Name = "courseforge-mcp"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.Logger == nil {
		cfg.Logger = defaultLogger{}
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://127.0.0.1:8080"
	}

	s := &Server{
		provider:          provider,
		session:           session,
		logger:            cfg.Logger,
		cfg:               cfg,
		currentCoursesDir: cfg.CoursesDir,
		currentDataDir:    cfg.DataDir,
		lastServerOnline:  provider != nil && session != nil,
	}
	if s.lastServerOnline {
		s.lastPing = time.Now()
	}

	mcpSrv := server.NewMCPServer(
		cfg.Name,
		cfg.Version,
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithResourceHandlerMiddleware(s.resourceMiddleware()),
	)
	s.mcpServer = mcpSrv
	s.mcpServer.Use(s.toolMiddleware())

	s.registerTools()
	s.registerResources()

	return s, nil
}

func (s *Server) toolMiddleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := s.ensureActiveServer(ctx); err != nil {
				return toolError("%v", err), nil
			}
			return next(ctx, req)
		}
	}
}

func (s *Server) resourceMiddleware() server.ResourceHandlerMiddleware {
	return func(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
		return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			if err := s.ensureActiveServer(ctx); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}

func (s *Server) ensureActiveServer(ctx context.Context) error {
	if !s.cfg.AutoDiscover {
		s.mu.RLock()
		prov := s.provider
		sess := s.session
		s.mu.RUnlock()
		if prov == nil || sess == nil {
			return errors.New("MCP provider is not configured")
		}
		return nil
	}

	serverURL := s.cfg.ServerURL
	if serverURL == "" {
		serverURL = "http://127.0.0.1:8080"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	coursesDir, dataDir, ok := DiscoverServerDirs(serverURL)
	s.lastPing = time.Now()
	s.lastServerOnline = ok

	if !ok {
		return fmt.Errorf("Основное приложение CourseForge не запущено (%s). Запустите CourseForge, чтобы использовать этот инструмент.", serverURL)
	}

	// Always verify that MCP is enabled in CourseForge settings
	mcpRepo := repo.NewMCPConfigRepository(dataDir)
	if mcpCfg, err := mcpRepo.Get(ctx); err == nil && mcpCfg != nil {
		if !mcpCfg.Enabled && !s.cfg.Force {
			return errors.New("MCP-сервер отключен в настройках CourseForge (Настройки -> MCP-сервер)")
		}
	}

	if s.provider == nil || s.session == nil || s.currentCoursesDir != coursesDir || s.currentDataDir != dataDir {
		if err := s.initDependenciesLocked(coursesDir, dataDir); err != nil {
			return fmt.Errorf("ошибка инициализации контекста CourseForge: %w", err)
		}
	}

	return nil
}

func (s *Server) initDependenciesLocked(coursesDir, dataDir string) error {
	mcpRepo := repo.NewMCPConfigRepository(dataDir)
	if mcpCfg, err := mcpRepo.Get(context.Background()); err == nil && mcpCfg != nil {
		if !mcpCfg.Enabled && !s.cfg.Force {
			return errors.New("MCP-сервер отключен в настройках CourseForge (Настройки -> MCP-сервер)")
		}
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := os.MkdirAll(coursesDir, 0755); err != nil {
		return fmt.Errorf("failed to create courses dir: %w", err)
	}

	dbPath := s.cfg.DBPath
	if dbPath == "" || s.cfg.AutoDiscover {
		dbPath = config.DefaultDBPath(dataDir)
	}

	db, err := repo.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("open submissions db: %w", err)
	}

	subRepo := repo.NewSubmissionRepository(db)
	progRepo := repo.NewFileProgressRepository(coursesDir)

	r := runner.New()
	runnersJSON := s.cfg.RunnersJSON
	if runnersJSON == "" || s.cfg.AutoDiscover {
		runnersJSON = config.DefaultRunnersJSON(dataDir)
	}
	if err := r.UseFile(runnersJSON); err != nil {
		s.logger.Printf("warning: failed to load runners.json: %v", err)
	}

	provider, err := NewCourseForgeProvider(coursesDir, dataDir, progRepo, subRepo, r)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("init CourseForge provider: %w", err)
	}

	stateFile := filepath.Join(dataDir, "mcp_session.json")
	session, err := NewFileSessionManager(stateFile)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("init session manager: %w", err)
	}

	if s.dbCloser != nil {
		_ = s.dbCloser.Close()
	}
	s.dbCloser = db
	s.provider = provider
	s.session = session
	s.currentCoursesDir = coursesDir
	s.currentDataDir = dataDir

	s.logger.Printf("CourseForge MCP connected to server (courses: %s, data: %s)", coursesDir, dataDir)
	return nil
}

func (s *Server) getDeps() (Provider, SessionManager) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider, s.session
}

// MCPServer returns the underlying mark3labs MCPServer.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// Session returns the SessionManager used by this MCP server.
func (s *Server) Session() SessionManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

// Provider returns the Provider used by this MCP server.
func (s *Server) Provider() Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider
}

// Close closes any resources owned by Server.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dbCloser != nil {
		return s.dbCloser.Close()
	}
	return nil
}

// ServeStdio starts the MCP server over standard I/O (stdin/stdout).
func (s *Server) ServeStdio() error {
	return s.ServeStdioWithIO(context.Background(), os.Stdin, os.Stdout)
}

// ServeStdioWithIO starts the MCP server over custom I/O streams.
func (s *Server) ServeStdioWithIO(ctx context.Context, in io.Reader, out io.Writer) error {
	stdioServer := server.NewStdioServer(s.mcpServer)
	return stdioServer.Listen(ctx, in, out)
}

// NewSSEServer returns an HTTP handler for SSE connections.
func (s *Server) NewSSEServer(baseURL string, opts ...server.SSEOption) *server.SSEServer {
	allOpts := make([]server.SSEOption, 0, len(opts)+1)
	if baseURL != "" {
		allOpts = append(allOpts, server.WithBaseURL(baseURL))
	}
	allOpts = append(allOpts, opts...)
	return server.NewSSEServer(s.mcpServer, allOpts...)
}

// StartSSE starts an HTTP server serving SSE on the given address.
func (s *Server) StartSSE(addr string) error {
	baseURL := fmt.Sprintf("http://%s", addr)
	sseServer := s.NewSSEServer(baseURL)

	mux := http.NewServeMux()
	mux.Handle("/sse", sseServer.SSEHandler())
	mux.Handle("/message", sseServer.MessageHandler())

	s.logger.Printf("CourseForge MCP SSE listening on %s (endpoints: /sse, /message)", addr)
	return http.ListenAndServe(addr, mux)
}

// Helper for tool execution errors
func toolError(format string, a ...any) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(format, a...))
}

func toolJSON(v any) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultJSON(v)
}
