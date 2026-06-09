package progress

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Progress tracks completed tasks for a single course.
type Progress struct {
	CourseSlug     string          `json:"course_slug"`
	CompletedTasks map[string]bool `json:"completed_tasks"`
}

// Store reads and writes per-course progress.json files.
// Files live at {coursesDir}/{courseSlug}/progress.json.
type Store struct {
	mu         sync.Mutex
	coursesDir string
}

func NewStore(coursesDir string) *Store {
	return &Store{coursesDir: coursesDir}
}

// Load returns progress for a course. Returns empty Progress if file doesn't exist yet.
// courseDir is the path relative to coursesDir (may differ from courseSlug for catalog courses).
func (s *Store) Load(courseDir, courseSlug string) (*Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(courseDir, courseSlug)
}

// MarkDone marks taskSlug as completed and persists.
func (s *Store) MarkDone(courseDir, courseSlug, taskSlug string) error {
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
func (s *Store) MarkUndone(courseDir, courseSlug, taskSlug string) error {
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
func (s *Store) load(courseDir, courseSlug string) (*Progress, error) {
	path := s.progressPath(courseDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Progress{
			CourseSlug:     courseSlug,
			CompletedTasks: make(map[string]bool),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.CompletedTasks == nil {
		p.CompletedTasks = make(map[string]bool)
	}
	return &p, nil
}

// save writes progress atomically via rename. Must be called with mu held.
func (s *Store) save(courseDir string, p *Progress) error {
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

func (s *Store) progressPath(courseDir string) string {
	return filepath.Join(s.coursesDir, courseDir, "progress.json")
}
