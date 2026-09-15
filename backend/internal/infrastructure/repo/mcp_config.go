package repo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type MCPConfigRepository struct {
	mu       sync.RWMutex
	filePath string
}

func NewMCPConfigRepository(dataDir string) *MCPConfigRepository {
	return &MCPConfigRepository{
		filePath: filepath.Join(dataDir, "mcp_server_config.json"),
	}
}

func (r *MCPConfigRepository) Get(ctx context.Context) (*domain.MCPConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := os.ReadFile(r.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return &domain.MCPConfig{
			Enabled:    true,
			Transport:  "stdio",
			Host:       "127.0.0.1",
			Port:       8085,
			CoursesDir: "./courses",
			DataDir:    "./data",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg domain.MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 8085
	}

	return &cfg, nil
}

func (r *MCPConfigRepository) Save(ctx context.Context, cfg *domain.MCPConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return err
	}

	tmp := r.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, r.filePath)
}
