package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestFileProgressRepository(t *testing.T) (*FileProgressRepository, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/go-basics", 0755); err != nil {
		t.Fatal(err)
	}
	return NewFileProgressRepository(dir), dir
}

func TestLoad_Empty(t *testing.T) {
	s, _ := newTestFileProgressRepository(t)
	p, err := s.Load(context.Background(), "go-basics", "go-basics")
	if err != nil {
		t.Fatal(err)
	}
	if p.CourseSlug != "go-basics" {
		t.Fatalf("expected slug 'go-basics', got %q", p.CourseSlug)
	}
	if len(p.CompletedTasks) != 0 {
		t.Fatalf("expected empty tasks, got %v", p.CompletedTasks)
	}
}

func TestMarkDone_Persist(t *testing.T) {
	s, _ := newTestFileProgressRepository(t)

	if err := s.MarkDone(context.Background(), "go-basics", "go-basics", "task-1"); err != nil {
		t.Fatal(err)
	}

	p, err := s.Load(context.Background(), "go-basics", "go-basics")
	if err != nil {
		t.Fatal(err)
	}
	if !p.CompletedTasks["task-1"] {
		t.Fatal("task-1 should be completed")
	}
}

func TestMarkUndone(t *testing.T) {
	s, _ := newTestFileProgressRepository(t)

	_ = s.MarkDone(context.Background(), "go-basics", "go-basics", "task-1")
	_ = s.MarkDone(context.Background(), "go-basics", "go-basics", "task-2")

	if err := s.MarkUndone(context.Background(), "go-basics", "go-basics", "task-1"); err != nil {
		t.Fatal(err)
	}

	p, err := s.Load(context.Background(), "go-basics", "go-basics")
	if err != nil {
		t.Fatal(err)
	}
	if p.CompletedTasks["task-1"] {
		t.Fatal("task-1 should not be completed")
	}
	if !p.CompletedTasks["task-2"] {
		t.Fatal("task-2 should still be completed")
	}
}

func TestSave_Atomic(t *testing.T) {
	s, dir := newTestFileProgressRepository(t)

	_ = s.MarkDone(context.Background(), "go-basics", "go-basics", "task-1")

	if _, err := os.Stat(filepath.Join(dir, "progress", "go-basics", "progress.json.tmp")); !os.IsNotExist(err) {
		t.Fatal("tmp file should not exist after save")
	}
}

func TestProgressStoredInDataDir(t *testing.T) {
	dataDir := t.TempDir()
	r := NewFileProgressRepository(dataDir)
	if err := r.MarkDone(context.Background(), "go-basics", "go-basics", "task-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "progress", "go-basics", "progress.json")); err != nil {
		t.Fatalf("progress file not in dataDir: %v", err)
	}
}

func TestMigrateProgress(t *testing.T) {
	coursesDir, dataDir := t.TempDir(), t.TempDir()
	legacy := filepath.Join(coursesDir, "cat", "course1")
	os.MkdirAll(legacy, 0755)
	os.WriteFile(filepath.Join(legacy, "progress.json"), []byte(`{"course_slug":"course1","completed_tasks":{"a":true}}`), 0644)
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "progress", "cat", "course1", "progress.json")); err != nil {
		t.Fatalf("not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "progress.json")); !os.IsNotExist(err) {
		t.Fatal("legacy file not removed")
	}
	// idempotent: second run must not error and must not resurrect
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	// never overwrite an existing new-location file
	os.WriteFile(filepath.Join(legacy, "progress.json"), []byte(`{}`), 0644)
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dataDir, "progress", "cat", "course1", "progress.json"))
	if !strings.Contains(string(data), "course1") {
		t.Fatal("migration overwrote existing progress")
	}
}

func TestMigrateProgressSkipsGitDirs(t *testing.T) {
	coursesDir, dataDir := t.TempDir(), t.TempDir()
	gitDir := filepath.Join(coursesDir, "course1", ".git", "refs")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "progress.json"), []byte(`{}`), 0644)
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "progress")); !os.IsNotExist(err) {
		t.Fatal("nothing should be migrated from .git")
	}
}
