package repo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// CourseSource records where a git-imported course/catalog came from.
type CourseSource struct {
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	ImportedAt string `json:"imported_at"` // RFC3339, set by caller
}

// CourseSourcesRepository persists {dataDir}/course_sources.json:
// course/catalog top-level dir → source. Atomic writes, in-process mutex.
type CourseSourcesRepository struct {
	mu   sync.Mutex
	dir  string
	path string
}

func NewCourseSourcesRepository(dataDir string) *CourseSourcesRepository {
	return &CourseSourcesRepository{
		dir:  dataDir,
		path: filepath.Join(dataDir, "course_sources.json"),
	}
}

func (r *CourseSourcesRepository) All(ctx context.Context) (map[string]CourseSource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadLocked(ctx)
}

func (r *CourseSourcesRepository) loadLocked(ctx context.Context) (map[string]CourseSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]CourseSource{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]CourseSource{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (r *CourseSourcesRepository) Set(ctx context.Context, courseDir string, s CourseSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	m, err := r.loadLocked(ctx)
	if err != nil {
		return err
	}
	m[courseDir] = s
	return r.saveLocked(m)
}

func (r *CourseSourcesRepository) Delete(ctx context.Context, courseDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	m, err := r.loadLocked(ctx)
	if err != nil {
		return err
	}
	if _, ok := m[courseDir]; !ok {
		return nil
	}
	delete(m, courseDir)
	return r.saveLocked(m)
}

func (r *CourseSourcesRepository) saveLocked(m map[string]CourseSource) error {
	data, err := json.MarshalIndent(m, "", "  ")
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
