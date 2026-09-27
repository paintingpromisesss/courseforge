package service

import (
	"context"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"go.uber.org/zap"
)

type fakeProgressRepo struct {
	marked int
}

func (f *fakeProgressRepo) Load(ctx context.Context, courseDir, courseSlug string) (*domain.Progress, error) {
	return &domain.Progress{CourseSlug: courseSlug}, nil
}
func (f *fakeProgressRepo) MarkDone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	f.marked++
	return nil
}
func (f *fakeProgressRepo) MarkUndone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	return nil
}
func (f *fakeProgressRepo) Reset(ctx context.Context, courseDir string) error { return nil }

func TestProgressOnChangeFired(t *testing.T) {
	repo := &fakeProgressRepo{}
	s := NewProgressService(repo, zap.NewNop())

	called := 0
	s.SetOnChange(func() { called++ })

	if err := s.MarkDone(context.Background(), "c1", "c1", "task-1"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("onChange called %d times, want 1", called)
	}

	// without a callback set on a fresh service: no panic
	s2 := NewProgressService(repo, zap.NewNop())
	if err := s2.MarkDone(context.Background(), "c1", "c1", "task-1"); err != nil {
		t.Fatal(err)
	}
}
