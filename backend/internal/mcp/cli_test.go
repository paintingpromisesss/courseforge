package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func TestMCPConfigDisabledCheck(t *testing.T) {
	tempDir := t.TempDir()
	mcpRepo := repo.NewMCPConfigRepository(tempDir)

	// Save disabled config
	err := mcpRepo.Save(context.Background(), &domain.MCPConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("failed to save mcp config: %v", err)
	}

	cfg, err := mcpRepo.Get(context.Background())
	if err != nil {
		t.Fatalf("failed to get mcp config: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("expected enabled=false, got %v", cfg.Enabled)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tempDir, "mcp_server_config.json")); err != nil {
		t.Errorf("expected mcp_server_config.json to exist: %v", err)
	}
}
