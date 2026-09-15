package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func TestMCPConfigHandlers(t *testing.T) {
	tempDir := t.TempDir()
	mcpRepo := repo.NewMCPConfigRepository(tempDir)
	h := New(filepath.Join(tempDir, "courses"), tempDir, nil, nil, nil, nil, nil, nil, mcpRepo)

	// Test GET /mcp/config
	req := httptest.NewRequest(http.MethodGet, "/mcp/config", nil)
	w := httptest.NewRecorder()
	h.getMCPConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var status domain.MCPStatusResponse
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

	// Test PATCH /mcp/config
	patchBody := domain.MCPConfig{
		Enabled:   false,
		Transport: "sse",
		Port:      8099,
	}
	bodyBytes, _ := json.Marshal(patchBody)
	patchReq := httptest.NewRequest(http.MethodPatch, "/mcp/config", bytes.NewReader(bodyBytes))
	patchW := httptest.NewRecorder()
	h.patchMCPConfig(patchW, patchReq)

	if patchW.Code != http.StatusOK {
		t.Fatalf("expected status 200 on patch, got %d: %s", patchW.Code, patchW.Body.String())
	}

	// Verify GET reflects changes
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
	if status2.Transport != "sse" {
		t.Errorf("expected transport=sse after patch, got %s", status2.Transport)
	}
	if status2.Port != 8099 {
		t.Errorf("expected port=8099 after patch, got %d", status2.Port)
	}
}
