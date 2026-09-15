package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the mark3labs MCP server with CourseForge specific dependencies.
type Server struct {
	mcpServer *server.MCPServer
	provider  Provider
	session   SessionManager
	logger    Logger
}

// Logger abstracts logging for the MCP server.
type Logger interface {
	Printf(format string, v ...any)
}

type defaultLogger struct{}

func (defaultLogger) Printf(format string, v ...any) {}

// Config holds configuration for the MCP Server.
type Config struct {
	Name        string
	Version     string
	CoursesDir  string
	DataDir     string
	DBPath      string
	RunnersJSON string
	Logger      Logger
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

	mcpSrv := server.NewMCPServer(
		cfg.Name,
		cfg.Version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	s := &Server{
		mcpServer: mcpSrv,
		provider:  provider,
		session:   session,
		logger:    cfg.Logger,
	}

	s.registerTools()
	s.registerResources()

	return s, nil
}

// MCPServer returns the underlying mark3labs MCPServer.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// ServeStdio starts the MCP server over standard I/O (stdin/stdout).
func (s *Server) ServeStdio() error {
	stdioServer := server.NewStdioServer(s.mcpServer)
	return stdioServer.Listen(context.Background(), os.Stdin, os.Stdout)
}

// SSEServer returns an HTTP handler for SSE connections.
func (s *Server) NewSSEServer(baseURL string) *server.SSEServer {
	return server.NewSSEServer(s.mcpServer, server.WithBaseURL(baseURL))
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
