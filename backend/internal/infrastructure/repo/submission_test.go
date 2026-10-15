package repo

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

func TestSubmissionRepository_ExportAndImport(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_submissions.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	r := NewSubmissionRepository(db)
	ctx := context.Background()

	t1 := time.Date(2026, 10, 15, 20, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 15, 20, 5, 0, 0, time.UTC)

	sub1 := &domain.Submission{
		CourseSlug:  "course-1",
		TaskSlug:    "task-a",
		Language:    "go",
		Code:        "package main\nfunc A() {}",
		PassedTests: 2,
		TotalTests:  2,
		CreatedAt:   t1,
	}
	sub2 := &domain.Submission{
		CourseSlug:  "course-1",
		TaskSlug:    "task-b",
		Language:    "go",
		Code:        "package main\nfunc B() {}",
		PassedTests: 1,
		TotalTests:  2,
		CreatedAt:   t2,
	}

	if _, err := r.Insert(ctx, sub1); err != nil {
		t.Fatalf("Insert sub1: %v", err)
	}
	if _, err := r.Insert(ctx, sub2); err != nil {
		t.Fatalf("Insert sub2: %v", err)
	}

	// Export course-1
	exported, err := r.ExportCourse(ctx, "course-1")
	if err != nil {
		t.Fatalf("ExportCourse: %v", err)
	}
	if len(exported) != 2 {
		t.Fatalf("expected 2 submissions, got %d", len(exported))
	}
	if exported[0].TaskSlug != "task-a" || exported[1].TaskSlug != "task-b" {
		t.Errorf("unexpected export order or items: %+v", exported)
	}

	// Now test import into a brand new DB
	dbPath2 := filepath.Join(t.TempDir(), "target.db")
	db2, err := NewDB(dbPath2)
	if err != nil {
		t.Fatalf("NewDB target: %v", err)
	}
	defer db2.Close()

	r2 := NewSubmissionRepository(db2)
	if err := r2.ImportSubmissions(ctx, exported); err != nil {
		t.Fatalf("ImportSubmissions: %v", err)
	}

	// Verify imported records
	importedA, err := r2.List(ctx, "course-1", "task-a")
	if err != nil || len(importedA) != 1 {
		t.Fatalf("expected 1 imported task-a, got %v (err: %v)", len(importedA), err)
	}
	if importedA[0].Code != sub1.Code {
		t.Errorf("expected code %q, got %q", sub1.Code, importedA[0].Code)
	}

	// Re-importing same submissions must be idempotent (no duplicates inserted)
	if err := r2.ImportSubmissions(ctx, exported); err != nil {
		t.Fatalf("Second ImportSubmissions: %v", err)
	}
	importedAAfter, _ := r2.List(ctx, "course-1", "task-a")
	if len(importedAAfter) != 1 {
		t.Errorf("expected still 1 task-a after duplicate import, got %d", len(importedAAfter))
	}
}

// Without busy_timeout ~half of these failed with SQLITE_BUSY.
func TestSubmissionRepository_ConcurrentWrites(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewSubmissionRepository(db)
	ctx := context.Background()

	errs := make(chan error, 20*25*2)
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				_, err := r.Insert(ctx, &domain.Submission{CourseSlug: "c", TaskSlug: "t", Language: "go", Code: "x", CreatedAt: time.Now()})
				errs <- err
				_, err = r.List(ctx, "c", "t")
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
