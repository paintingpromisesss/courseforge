package repo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// SyncTriggers control when the sync engine pushes/pulls automatically.
type SyncTriggers struct {
	OnProgress    bool `json:"on_progress"`
	IntervalMin   int  `json:"interval_min"` // 0 = off
	OnStartupPull bool `json:"on_startup_pull"`
}

// SyncConfig is the cloud-sync setup for the courses directory.
type SyncConfig struct {
	RemoteURL string       `json:"remote_url"`
	Branch    string       `json:"branch"`
	Triggers  SyncTriggers `json:"triggers"`
	LastSync  string       `json:"last_sync,omitempty"` // RFC3339
	Enabled   bool         `json:"enabled"`
	// Exclude lists courseDir values kept local-only (opt-out).
	// Git-imported courses are excluded implicitly — they have their own source repo.
	Exclude []string `json:"exclude,omitempty"`
}

// SyncConfigRepository persists {dataDir}/sync_config.json.
// Atomic writes, in-process mutex. Absent file → (nil, nil); callers apply defaults.
type SyncConfigRepository struct {
	mu   sync.Mutex
	dir  string
	path string
}

func NewSyncConfigRepository(dataDir string) *SyncConfigRepository {
	return &SyncConfigRepository{
		dir:  dataDir,
		path: filepath.Join(dataDir, "sync_config.json"),
	}
}

func (r *SyncConfigRepository) Load(ctx context.Context) (*SyncConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	var cfg SyncConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *SyncConfigRepository) Save(ctx context.Context, cfg *SyncConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}
