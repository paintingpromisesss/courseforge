package repo

import (
	"context"
	"os"
	"testing"
	"time"
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

func TestLoad_CachePicksUpExternalWrite(t *testing.T) {
	s, dir := newTestFileProgressRepository(t)
	s.revalidate = 0
	ctx := context.Background()

	_ = s.MarkDone(ctx, "go-basics", "go-basics", "task-1")
	if _, err := s.Load(ctx, "go-basics", "go-basics"); err != nil { // warm cache
		t.Fatal(err)
	}

	// Another process (MCP server) rewrites the file.
	ext := `{"course_slug":"go-basics","completed_tasks":{"task-1":true,"task-2":true}}`
	if err := os.WriteFile(dir+"/go-basics/progress.json", []byte(ext), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := s.Load(ctx, "go-basics", "go-basics")
	if err != nil {
		t.Fatal(err)
	}
	if !p.CompletedTasks["task-2"] {
		t.Fatal("external write not picked up")
	}

	// Mutating the returned copy must not leak into the cache.
	p.CompletedTasks["task-3"] = true
	p2, _ := s.Load(ctx, "go-basics", "go-basics")
	if p2.CompletedTasks["task-3"] {
		t.Fatal("caller mutation leaked into cache")
	}

	if err := os.Remove(dir + "/go-basics/progress.json"); err != nil {
		t.Fatal(err)
	}
	p3, _ := s.Load(ctx, "go-basics", "go-basics")
	if len(p3.CompletedTasks) != 0 {
		t.Fatal("deleted file still served from cache")
	}
}

// Reads may be stale within the revalidate interval, but a mutation must not
// overwrite a newer file written by another process.
func TestMarkDone_DoesNotLoseExternalWrite(t *testing.T) {
	s, dir := newTestFileProgressRepository(t)
	s.revalidate = time.Hour
	ctx := context.Background()

	_ = s.MarkDone(ctx, "go-basics", "go-basics", "task-1")
	_, _ = s.Load(ctx, "go-basics", "go-basics") // warm cache

	ext := `{"course_slug":"go-basics","completed_tasks":{"task-1":true,"task-2":true}}`
	if err := os.WriteFile(dir+"/go-basics/progress.json", []byte(ext), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDone(ctx, "go-basics", "go-basics", "task-3"); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Load(ctx, "go-basics", "go-basics")
	for _, slug := range []string{"task-1", "task-2", "task-3"} {
		if !p.CompletedTasks[slug] {
			t.Fatalf("%s lost: %v", slug, p.CompletedTasks)
		}
	}
}

func TestSave_Atomic(t *testing.T) {
	s, dir := newTestFileProgressRepository(t)

	_ = s.MarkDone(context.Background(), "go-basics", "go-basics", "task-1")

	if _, err := os.Stat(dir + "/go-basics/progress.json.tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file should not exist after save")
	}
}
