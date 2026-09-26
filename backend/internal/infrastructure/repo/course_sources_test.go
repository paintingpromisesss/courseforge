package repo

import (
	"context"
	"sync"
	"testing"
)

func TestCourseSourcesRoundTrip(t *testing.T) {
	r := NewCourseSourcesRepository(t.TempDir())
	ctx := context.Background()

	all, err := r.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty store, got %v", all)
	}

	src := CourseSource{Repo: "https://github.com/o/r", Branch: "main", Commit: "abc123", ImportedAt: "2026-09-27T00:00:00Z"}
	if err := r.Set(ctx, "go-interview", src); err != nil {
		t.Fatal(err)
	}
	src2 := CourseSource{Repo: "https://github.com/o/r2", Branch: "dev", Commit: "def456"}
	if err := r.Set(ctx, "cat/alogue", src2); err != nil {
		t.Fatal(err)
	}

	// A fresh instance must see the persisted state.
	r2 := NewCourseSourcesRepository(r.dir)
	all, err = r2.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}
	if all["go-interview"] != src {
		t.Fatalf("go-interview = %+v, want %+v", all["go-interview"], src)
	}

	// Overwrite existing key.
	src.Commit = "new"
	if err := r2.Set(ctx, "go-interview", src); err != nil {
		t.Fatal(err)
	}
	all, _ = r.All(ctx)
	if all["go-interview"].Commit != "new" {
		t.Fatal("Set did not overwrite")
	}

	if err := r.Delete(ctx, "go-interview"); err != nil {
		t.Fatal(err)
	}
	// Deleting a missing key is not an error.
	if err := r.Delete(ctx, "nope"); err != nil {
		t.Fatal(err)
	}
	all, _ = r.All(ctx)
	if len(all) != 1 {
		t.Fatalf("len after delete = %d, want 1", len(all))
	}
}

func TestCourseSourcesConcurrentSet(t *testing.T) {
	r := NewCourseSourcesRepository(t.TempDir())
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Set(ctx, string(rune('a'+i)), CourseSource{Repo: "r"})
		}()
	}
	wg.Wait()
	all, err := r.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 8 {
		t.Fatalf("len = %d, want 8", len(all))
	}
}
