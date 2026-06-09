package progress

import (
	"os"
	"testing"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/go-basics", 0755); err != nil {
		t.Fatal(err)
	}
	return NewStore(dir), dir
}

func TestLoad_Empty(t *testing.T) {
	s, _ := newTestStore(t)
	p, err := s.Load("go-basics", "go-basics")
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
	s, _ := newTestStore(t)

	if err := s.MarkDone("go-basics", "go-basics", "task-1"); err != nil {
		t.Fatal(err)
	}

	p, err := s.Load("go-basics", "go-basics")
	if err != nil {
		t.Fatal(err)
	}
	if !p.CompletedTasks["task-1"] {
		t.Fatal("task-1 should be completed")
	}
}

func TestMarkUndone(t *testing.T) {
	s, _ := newTestStore(t)

	_ = s.MarkDone("go-basics", "go-basics", "task-1")
	_ = s.MarkDone("go-basics", "go-basics", "task-2")

	if err := s.MarkUndone("go-basics", "go-basics", "task-1"); err != nil {
		t.Fatal(err)
	}

	p, err := s.Load("go-basics", "go-basics")
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
	s, dir := newTestStore(t)

	_ = s.MarkDone("go-basics", "go-basics", "task-1")

	if _, err := os.Stat(dir + "/go-basics/progress.json.tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file should not exist after save")
	}
}
