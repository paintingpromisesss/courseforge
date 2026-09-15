package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ActiveTaskContext represents the active task tracked for the session.
type ActiveTaskContext struct {
	CourseSlug string    `json:"course_slug"`
	TaskSlug   string    `json:"task_slug"`
	Language   string    `json:"language,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SessionManager manages the user's active task context.
type SessionManager interface {
	GetActiveTask(ctx context.Context) (*ActiveTaskContext, error)
	SetActiveTask(ctx context.Context, courseSlug, taskSlug, language string) (*ActiveTaskContext, error)
	ClearActiveTask(ctx context.Context) error
}

// FileSessionManager stores the active task context in memory with an optional backup JSON file.
type FileSessionManager struct {
	mu          sync.RWMutex
	filePath    string
	active      *ActiveTaskContext
	lastModTime time.Time
}

// NewFileSessionManager creates a SessionManager backed by memory and an optional state file.
func NewFileSessionManager(filePath string) (*FileSessionManager, error) {
	sm := &FileSessionManager{
		filePath: filePath,
	}

	if filePath != "" {
		if fi, err := os.Stat(filePath); err == nil {
			sm.lastModTime = fi.ModTime()
			if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
				var state ActiveTaskContext
				if err := json.Unmarshal(data, &state); err == nil && state.CourseSlug != "" && state.TaskSlug != "" {
					sm.active = &state
				}
			}
		}
	}

	return sm, nil
}

func (sm *FileSessionManager) reloadIfNeeded() {
	if sm.filePath == "" {
		return
	}
	fi, err := os.Stat(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) && sm.active != nil {
			sm.active = nil
			sm.lastModTime = time.Time{}
		}
		return
	}
	if fi.ModTime().Equal(sm.lastModTime) {
		return
	}

	data, err := os.ReadFile(sm.filePath)
	if err != nil || len(data) == 0 {
		return
	}
	var state ActiveTaskContext
	if err := json.Unmarshal(data, &state); err == nil && state.CourseSlug != "" && state.TaskSlug != "" {
		sm.active = &state
		sm.lastModTime = fi.ModTime()
	}
}

func (sm *FileSessionManager) GetActiveTask(ctx context.Context) (*ActiveTaskContext, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.reloadIfNeeded()

	if sm.active == nil {
		return nil, nil
	}
	// Return a copy
	cp := *sm.active
	return &cp, nil
}

func (sm *FileSessionManager) SetActiveTask(ctx context.Context, courseSlug, taskSlug, language string) (*ActiveTaskContext, error) {
	if courseSlug == "" || taskSlug == "" {
		return nil, errors.New("course_slug and task_slug are required")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	ctxObj := &ActiveTaskContext{
		CourseSlug: courseSlug,
		TaskSlug:   taskSlug,
		Language:   language,
		UpdatedAt:  time.Now().UTC(),
	}
	sm.active = ctxObj

	if sm.filePath != "" {
		if err := os.MkdirAll(filepath.Dir(sm.filePath), 0755); err == nil {
			if data, err := json.MarshalIndent(ctxObj, "", "  "); err == nil {
				if err := os.WriteFile(sm.filePath, data, 0644); err == nil {
					if fi, err := os.Stat(sm.filePath); err == nil {
						sm.lastModTime = fi.ModTime()
					}
				}
			}
		}
	}

	cp := *ctxObj
	return &cp, nil
}

func (sm *FileSessionManager) ClearActiveTask(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.active = nil
	sm.lastModTime = time.Time{}
	if sm.filePath != "" {
		_ = os.Remove(sm.filePath)
	}
	return nil
}

// Ensure interface implementation
var _ SessionManager = (*FileSessionManager)(nil)

