package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func TestMCPConfigHandlers(t *testing.T) {
	tempDir := t.TempDir()
	mcpRepo := repo.NewMCPConfigRepository(tempDir)
	h := New(filepath.Join(tempDir, "courses"), tempDir, nil, nil, nil, nil, nil, nil, mcpRepo, nil)

	// Test GET /mcp/config
	req := httptest.NewRequest(http.MethodGet, "/mcp/config", nil)
	w := httptest.NewRecorder()
	h.getMCPConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var status dto.MCPStatusResp
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !status.Enabled {
		t.Errorf("expected default enabled=true, got %v", status.Enabled)
	}
	if status.Transport != "stdio" {
		t.Errorf("expected transport=stdio, got %s", status.Transport)
	}
	if status.ToolsCount != 9 {
		t.Errorf("expected 9 tools, got %d", status.ToolsCount)
	}
	if status.Command == "" {
		t.Errorf("expected command to be populated, got empty")
	}
	if len(status.Args) == 0 {
		t.Errorf("expected args to be populated, got empty")
	}

	// Test PATCH /mcp/config (disable MCP)
	disabled := false
	patchBody := dto.MCPConfigReq{
		Enabled: &disabled,
	}
	bodyBytes, _ := json.Marshal(patchBody)
	patchReq := httptest.NewRequest(http.MethodPatch, "/mcp/config", bytes.NewReader(bodyBytes))
	patchW := httptest.NewRecorder()
	h.patchMCPConfig(patchW, patchReq)

	if patchW.Code != http.StatusOK {
		t.Fatalf("expected status 200 on patch, got %d: %s", patchW.Code, patchW.Body.String())
	}

	// Verify GET reflects disabled state
	req2 := httptest.NewRequest(http.MethodGet, "/mcp/config", nil)
	w2 := httptest.NewRecorder()
	h.getMCPConfig(w2, req2)

	var status2 domain.MCPStatusResponse
	if err := json.NewDecoder(w2.Body).Decode(&status2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if status2.Enabled != false {
		t.Errorf("expected enabled=false after patch, got %v", status2.Enabled)
	}

	// Verify SSE rejects connection when MCP is disabled
	sseReq := httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	sseW := httptest.NewRecorder()
	h.handleMCPSSE(sseW, sseReq)

	if sseW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for SSE when MCP disabled, got %d", sseW.Code)
	}
}

func TestMCPActiveTaskHandlers(t *testing.T) {
	tempDir := t.TempDir()
	h := New(filepath.Join(tempDir, "courses"), tempDir, nil, nil, nil, nil, nil, nil, nil, nil)

	// 1. Initial GET -> empty
	req := httptest.NewRequest(http.MethodGet, "/mcp/active-task", nil)
	w := httptest.NewRecorder()
	h.getMCPActiveTask(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp dto.MCPActiveTaskResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.CourseSlug != "" || resp.TaskSlug != "" {
		t.Fatalf("expected empty active task, got %+v", resp)
	}

	// 2. PUT /mcp/active-task
	putBody := dto.MCPActiveTaskReq{
		CourseSlug: "go-course",
		TaskSlug:   "hello-world",
		Language:   "go",
	}
	bodyBytes, _ := json.Marshal(putBody)
	putReq := httptest.NewRequest(http.MethodPut, "/mcp/active-task", bytes.NewReader(bodyBytes))
	putW := httptest.NewRecorder()
	h.putMCPActiveTask(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("expected 200 on put, got %d: %s", putW.Code, putW.Body.String())
	}

	// 3. GET /mcp/active-task after PUT
	req2 := httptest.NewRequest(http.MethodGet, "/mcp/active-task", nil)
	w2 := httptest.NewRecorder()
	h.getMCPActiveTask(w2, req2)
	var resp2 dto.MCPActiveTaskResp
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp2.CourseSlug != "go-course" || resp2.TaskSlug != "hello-world" || resp2.Language != "go" {
		t.Fatalf("unexpected active task: %+v", resp2)
	}

	// 4. DELETE /mcp/active-task
	delReq := httptest.NewRequest(http.MethodDelete, "/mcp/active-task", nil)
	delW := httptest.NewRecorder()
	h.deleteMCPActiveTask(delW, delReq)
	if delW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delW.Code)
	}

	// 5. GET /mcp/active-task after DELETE
	req3 := httptest.NewRequest(http.MethodGet, "/mcp/active-task", nil)
	w3 := httptest.NewRecorder()
	h.getMCPActiveTask(w3, req3)
	var resp3 dto.MCPActiveTaskResp
	if err := json.NewDecoder(w3.Body).Decode(&resp3); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp3.CourseSlug != "" || resp3.TaskSlug != "" {
		t.Fatalf("expected empty active task after delete, got %+v", resp3)
	}
}

func TestFindMCPCommand(t *testing.T) {
	cmd, args, ok := findMCPCommand("./courses", "./data")
	if !filepath.IsAbs(cmd) {
		t.Errorf("expected absolute command path, got %q", cmd)
	}
	if len(args) != 1 || args[0] != "mcp" {
		t.Errorf("expected ['mcp'] args, got %v", args)
	}
	t.Logf("found cmd: %s (available: %v)", cmd, ok)
}

