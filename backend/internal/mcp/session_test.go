package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func TestFileSessionManager_MemoryAndFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mcp-session-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stateFile := filepath.Join(tmpDir, "active_task.json")
	ctx := context.Background()

	// 1. New manager, empty state
	sm, err := mcp.NewFileSessionManager(stateFile)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	active, err := sm.GetActiveTask(ctx)
	if err != nil {
		t.Fatalf("get active task: %v", err)
	}
	if active != nil {
		t.Fatalf("expected nil active task, got %+v", active)
	}

	// 2. Set active task
	setResult, err := sm.SetActiveTask(ctx, "go-interview", "merge-sorted", "go")
	if err != nil {
		t.Fatalf("set active task: %v", err)
	}
	if setResult.CourseSlug != "go-interview" || setResult.TaskSlug != "merge-sorted" || setResult.Language != "go" {
		t.Fatalf("unexpected set result: %+v", setResult)
	}

	// Verify persistence to file
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file was not created: %v", err)
	}

	// 3. New manager instance reading existing state file
	sm2, err := mcp.NewFileSessionManager(stateFile)
	if err != nil {
		t.Fatalf("new session manager 2: %v", err)
	}
	active2, err := sm2.GetActiveTask(ctx)
	if err != nil {
		t.Fatalf("get active task from restored session: %v", err)
	}
	if active2 == nil || active2.CourseSlug != "go-interview" || active2.TaskSlug != "merge-sorted" {
		t.Fatalf("failed to restore active task: %+v", active2)
	}

	// 4. Clear active task
	if err := sm2.ClearActiveTask(ctx); err != nil {
		t.Fatalf("clear active task: %v", err)
	}
	active3, err := sm2.GetActiveTask(ctx)
	if err != nil {
		t.Fatalf("get active task after clear: %v", err)
	}
	if active3 != nil {
		t.Fatalf("expected nil active task after clear, got %+v", active3)
	}
}

func TestFileSessionManager_ExternalUpdateReload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mcp-session-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stateFile := filepath.Join(tmpDir, "active_task.json")
	ctx := context.Background()

	// sm1 simulates CourseForge server process
	sm1, err := mcp.NewFileSessionManager(stateFile)
	if err != nil {
		t.Fatalf("sm1 init: %v", err)
	}

	// sm2 simulates external stdio agent process (e.g. OpenCode)
	sm2, err := mcp.NewFileSessionManager(stateFile)
	if err != nil {
		t.Fatalf("sm2 init: %v", err)
	}

	// Initially empty
	act, err := sm2.GetActiveTask(ctx)
	if err != nil || act != nil {
		t.Fatalf("expected nil active, got %+v (err: %v)", act, err)
	}

	// CourseForge server updates active task
	if _, err := sm1.SetActiveTask(ctx, "course-1", "task-1", "go"); err != nil {
		t.Fatalf("sm1 SetActiveTask: %v", err)
	}

	// sm2 should immediately see the update on GetActiveTask without restart
	act2, err := sm2.GetActiveTask(ctx)
	if err != nil {
		t.Fatalf("sm2 GetActiveTask: %v", err)
	}
	if act2 == nil || act2.CourseSlug != "course-1" || act2.TaskSlug != "task-1" {
		t.Fatalf("sm2 failed to see update: %+v", act2)
	}

	// CourseForge server clears active task
	if err := sm1.ClearActiveTask(ctx); err != nil {
		t.Fatalf("sm1 ClearActiveTask: %v", err)
	}

	act3, err := sm2.GetActiveTask(ctx)
	if err != nil || act3 != nil {
		t.Fatalf("expected nil active after clear, got %+v (err: %v)", act3, err)
	}
}

