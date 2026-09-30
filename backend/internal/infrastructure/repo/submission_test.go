package repo

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

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
