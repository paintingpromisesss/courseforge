package repo

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

// FileProgressRepository reads and writes per-course progress.json files.
// Files live at {coursesDir}/{courseSlug}/progress.json.
type FileProgressRepository struct {
	mu         sync.Mutex
	coursesDir string
}

func NewFileProgressRepository(coursesDir string) *FileProgressRepository {
	return &FileProgressRepository{coursesDir: coursesDir}
}

// Load returns progress for a course. Returns empty Progress if file doesn't exist yet.
// courseDir is the path relative to coursesDir (may differ from courseSlug for catalog courses).
func (s *FileProgressRepository) Load(courseDir, courseSlug string) (*domain.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(courseDir, courseSlug)
}

// MarkDone marks taskSlug as completed and persists.
func (s *FileProgressRepository) MarkDone(courseDir, courseSlug, taskSlug string) error {
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
func (s *FileProgressRepository) MarkUndone(courseDir, courseSlug, taskSlug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(courseDir, courseSlug)
	if err != nil {
		return err
	}
	delete(p.CompletedTasks, taskSlug)
	return s.save(courseDir, p)
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
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *FileProgressRepository) progressPath(courseDir string) string {
	return filepath.Join(s.coursesDir, courseDir, "progress.json")
}
