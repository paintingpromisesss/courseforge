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
