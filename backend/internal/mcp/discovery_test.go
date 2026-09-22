package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func TestAutoDiscover_ServerOfflineReturnsError(t *testing.T) {
	ctx := context.Background()

	// Use an unused port on localhost
	srv, err := mcp.NewServer(mcp.Config{
		Name:         "courseforge-mcp-test",
		Version:      "1.0.0",
		ServerURL:    "http://127.0.0.1:59123",
		AutoDiscover: true,
	}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}

	// 1. Initialize should still succeed without crashing
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`
	resp := srv.MCPServer().HandleMessage(ctx, json.RawMessage(initReq))
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal init response: %v", err)
	}
	if !strings.Contains(string(respBytes), `"serverInfo"`) {
		t.Fatalf("expected serverInfo in initialize response, got: %s", string(respBytes))
	}

	// 2. Tools call should return tool error because CourseForge server is offline
	callReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(callReq))
	respBytes, err = json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal call response: %v", err)
	}
	respStr := string(respBytes)
	if !strings.Contains(respStr, "Основное приложение CourseForge не запущено") {
		t.Fatalf("expected offline error message, got: %s", respStr)
	}
	if !strings.Contains(respStr, `"isError":true`) {
		t.Fatalf("expected isError: true in tool call response, got: %s", respStr)
	}
}

func TestAutoDiscover_ServerLifecycleDynamic(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "mcp-autodiscover-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	coursesDir := filepath.Join(tmpDir, "courses")
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(coursesDir, 0755); err != nil {
		t.Fatalf("mkdir courses: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	var serverRunning atomic.Bool
	serverRunning.Store(false)

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !serverRunning.Load() {
			http.Error(w, "server stopped", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/api/info" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"courses_dir": coursesDir,
				"data_dir":    dataDir,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer httpServer.Close()

	srv, err := mcp.NewServer(mcp.Config{
		Name:         "courseforge-mcp-test",
		Version:      "1.0.0",
		ServerURL:    httpServer.URL,
		AutoDiscover: true,
	}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}
	defer srv.Close()

	// Phase 1: Server is offline
	callReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}`
	resp := srv.MCPServer().HandleMessage(ctx, json.RawMessage(callReq))
	respBytes, _ := json.Marshal(resp)
	if !strings.Contains(string(respBytes), "Основное приложение CourseForge не запущено") {
		t.Fatalf("phase 1: expected offline error, got: %s", string(respBytes))
	}

	// Phase 2: Server becomes online -> tool dynamically picks up directories and succeeds
	serverRunning.Store(true)
	callReq2 := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}`
	resp2 := srv.MCPServer().HandleMessage(ctx, json.RawMessage(callReq2))
	respBytes2, _ := json.Marshal(resp2)
	if strings.Contains(string(respBytes2), `"isError":true`) {
		t.Fatalf("phase 2: expected success when server is up, got: %s", string(respBytes2))
	}
	if !strings.Contains(string(respBytes2), `"courses"`) {
		t.Fatalf("phase 2: expected courses in response, got: %s", string(respBytes2))
	}

	// Phase 3: Server stops again -> after cache expires, tool returns offline error
	serverRunning.Store(false)
	time.Sleep(1100 * time.Millisecond) // wait for ping cache to expire

	callReq3 := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}`
	resp3 := srv.MCPServer().HandleMessage(ctx, json.RawMessage(callReq3))
	respBytes3, _ := json.Marshal(resp3)
	if !strings.Contains(string(respBytes3), "Основное приложение CourseForge не запущено") {
		t.Fatalf("phase 3: expected offline error when server stops, got: %s", string(respBytes3))
	}

	// Phase 4: Server comes back up -> tool immediately recovers
	serverRunning.Store(true)
	callReq4 := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}`
	resp4 := srv.MCPServer().HandleMessage(ctx, json.RawMessage(callReq4))
	respBytes4, _ := json.Marshal(resp4)
	if strings.Contains(string(respBytes4), `"isError":true`) {
		t.Fatalf("phase 4: expected recovery when server is up again, got: %s", string(respBytes4))
	}
}
