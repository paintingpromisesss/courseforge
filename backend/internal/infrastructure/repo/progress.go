package repo

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

// FileProgressRepository reads and writes per-course progress.json files.
// Files live at {coursesDir}/{courseSlug}/progress.json.
//
// Parsed files are cached and revalidated by mtime+size, so writes from another
// process (the MCP server) are still picked up. Reads revalidate at most every
// revalidate interval: os.Stat on Windows costs about as much as opening the file,
// and /api/courses loads every course's progress per request. Mutations always
// revalidate so they never overwrite a newer file.
type FileProgressRepository struct {
	mu         sync.Mutex
	coursesDir string
	revalidate time.Duration
	cache      map[string]cachedProgress // by file path
}

type cachedProgress struct {
	checked time.Time
	modTime time.Time
	size    int64
	p       *domain.Progress // nil: file doesn't exist
}

func NewFileProgressRepository(coursesDir string) *FileProgressRepository {
	return &FileProgressRepository{
		coursesDir: coursesDir,
		revalidate: time.Second,
		cache:      make(map[string]cachedProgress),
	}
}

// Load returns progress for a course. Returns empty Progress if file doesn't exist yet.
// courseDir is the path relative to coursesDir (may differ from courseSlug for catalog courses).
func (s *FileProgressRepository) Load(ctx context.Context, courseDir, courseSlug string) (*domain.Progress, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(courseDir, courseSlug, false)
}

// MarkDone marks taskSlug as completed and persists.
func (s *FileProgressRepository) MarkDone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(courseDir, courseSlug, true)
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

	p, err := s.load(courseDir, courseSlug, true)
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

	path := s.progressPath(courseDir)
	delete(s.cache, path)
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// load returns progress from cache, re-reading the file only if it changed on
// disk. fresh forces a stat even within the revalidate interval. The result is a
// copy the caller may mutate. Must be called with mu held.
func (s *FileProgressRepository) load(courseDir, courseSlug string, fresh bool) (*domain.Progress, error) {
	path := s.progressPath(courseDir)
	now := time.Now()

	c, ok := s.cache[path]
	if !ok || fresh || now.Sub(c.checked) >= s.revalidate {
		fi, err := os.Stat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			c = cachedProgress{checked: now}
		case err != nil:
			return nil, err
		case c.p == nil || !c.modTime.Equal(fi.ModTime()) || c.size != fi.Size():
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var p domain.Progress
			if err := json.Unmarshal(data, &p); err != nil {
				return nil, err
			}
			c = cachedProgress{checked: now, modTime: fi.ModTime(), size: fi.Size(), p: &p}
		default:
			c.checked = now
		}
		s.cache[path] = c
	}

	if c.p == nil {
		return &domain.Progress{
			CourseSlug:     courseSlug,
			CompletedTasks: make(map[string]bool),
		}, nil
	}
	p := *c.p
	p.CompletedTasks = maps.Clone(c.p.CompletedTasks)
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
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	delete(s.cache, path)
	return os.Rename(tmp, path)
}

func (s *FileProgressRepository) progressPath(courseDir string) string {
	return filepath.Join(s.coursesDir, courseDir, "progress.json")
}
