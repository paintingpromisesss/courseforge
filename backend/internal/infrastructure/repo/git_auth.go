package repo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// GitAuth holds the GitHub Personal Access Token used for private repos and
// cloud sync. Stored in {dataDir}/git_auth.json; never returned whole by the API.
type GitAuth struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

type GitAuthRepository struct {
	path string
}

func NewGitAuthRepository(dataDir string) *GitAuthRepository {
	return &GitAuthRepository{path: filepath.Join(dataDir, "git_auth.json")}
}

// Load returns (nil, nil) when no auth is configured.
func (r *GitAuthRepository) Load(ctx context.Context) (*GitAuth, error) {
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
	var a GitAuth
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *GitAuthRepository) Save(ctx context.Context, a *GitAuth) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func (r *GitAuthRepository) Delete() error {
	err := os.Remove(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// MaskToken renders a token as its first 4 and last 4 characters, e.g.
// "ghp_abcdefghijklmnop" → "ghp_…mnop". Short or empty tokens are fully masked.
func MaskToken(t string) string {
	if t == "" {
		return ""
	}
	if len(t) < 9 {
		return "****"
	}
	return t[:4] + "…" + t[len(t)-4:]
}
