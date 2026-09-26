package repo

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

// FileProgressRepository reads and writes per-course progress.json files.
// Files live at {dataDir}/progress/{courseDir}/progress.json — outside the
// courses tree so branch switches and cloud sync never touch user progress.
type FileProgressRepository struct {
	mu          sync.Mutex
	progressDir string
}

func NewFileProgressRepository(dataDir string) *FileProgressRepository {
	return &FileProgressRepository{progressDir: filepath.Join(dataDir, "progress")}
}

// MigrateProgress moves legacy {coursesDir}/**/progress.json files into
// {dataDir}/progress/, preserving relative paths. Idempotent; never
// overwrites a file that already exists at the destination.
func MigrateProgress(coursesDir, dataDir string) error {
	dest := filepath.Join(dataDir, "progress")
	return filepath.WalkDir(coursesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable course dirs are not our problem
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if d.IsDir() || d.Name() != "progress.json" {
			return nil
		}
		rel, _ := filepath.Rel(coursesDir, path)
		target := filepath.Join(dest, filepath.Dir(rel), "progress.json")
		if _, err := os.Stat(target); err == nil {
			return os.Remove(path) // destination wins; drop stale legacy copy
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.Rename(path, target); err != nil {
			// cross-device fallback
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if werr := os.WriteFile(target, data, 0644); werr != nil {
				return werr
			}
			return os.Remove(path)
		}
		return nil
	})
}

// Load returns progress for a course. Returns empty Progress if file doesn't exist yet.
// courseDir is the path relative to coursesDir (may differ from courseSlug for catalog courses).
func (s *FileProgressRepository) Load(ctx context.Context, courseDir, courseSlug string) (*domain.Progress, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(courseDir, courseSlug)
}

// MarkDone marks taskSlug as completed and persists.
func (s *FileProgressRepository) MarkDone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(courseDir, courseSlug)
	if err != nil {
		return err
	}
	p.CompletedTasks[taskSlug] = true
	return s.save(courseDir, p)
}

// MarkUndone marks taskSlug as not completed and persists.
func (s *FileProgressRepository) MarkUndone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(courseDir, courseSlug)
	if err != nil {
		return err
	}
	delete(p.CompletedTasks, taskSlug)
	return s.save(courseDir, p)
}

// Reset removes all progress for a course by deleting its progress.json;
// a subsequent Load returns empty progress.
func (s *FileProgressRepository) Reset(ctx context.Context, courseDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.progressPath(courseDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// load reads progress from disk. Must be called with mu held.
func (s *FileProgressRepository) load(courseDir, courseSlug string) (*domain.Progress, error) {
	path := s.progressPath(courseDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &domain.Progress{
			CourseSlug:     courseSlug,
			CompletedTasks: make(map[string]bool),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var p domain.Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.CompletedTasks == nil {
		p.CompletedTasks = make(map[string]bool)
	}
	return &p, nil
}

// save writes progress atomically via rename. Must be called with mu held.
func (s *FileProgressRepository) save(courseDir string, p *domain.Progress) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	path := s.progressPath(courseDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *FileProgressRepository) progressPath(courseDir string) string {
	return filepath.Join(s.progressDir, filepath.FromSlash(courseDir), "progress.json")
}
