package repo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type AIConfigRepository struct {
	path string
}

func NewAIConfigRepository(dataDir string) *AIConfigRepository {
	return &AIConfigRepository{
		path: filepath.Join(dataDir, "ai_config.json"),
	}
}

func (r *AIConfigRepository) Load(ctx context.Context) (*domain.AIConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg domain.AIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *AIConfigRepository) Save(ctx context.Context, cfg *domain.AIConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}
