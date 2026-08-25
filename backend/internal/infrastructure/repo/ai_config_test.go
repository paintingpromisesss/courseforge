package repo

import (
	"context"
	"os"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

func TestAIConfigRepository_LoadSave(t *testing.T) {
	dir := t.TempDir()
	r := NewAIConfigRepository(dir)

	// Test load nonexistent
	cfg, err := r.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error loading nonexistent: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got %v", cfg)
	}

	// Test save and load
	expected := &domain.AIConfig{
		Provider:     domain.ProviderOpenAI,
		BaseURL:      "http://localhost:11434/v1",
		APIKey:       "test-key",
		Model:        "llama3",
		SystemPrompt: "test prompt",
		Enabled:      true,
	}

	if err := r.Save(context.Background(), expected); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := r.Load(context.Background())
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil loaded config")
	}
	if loaded.Provider != expected.Provider || loaded.Model != expected.Model || !loaded.Enabled {
		t.Fatalf("loaded config mismatch: %+v", loaded)
	}

	// Tmp file cleaned up
	if _, err := os.Stat(r.path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file should not exist after save")
	}
}
