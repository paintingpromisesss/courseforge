package service

import (
	"context"
	"fmt"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"go.uber.org/zap"
)

type progressRepository interface {
	Load(ctx context.Context, courseDir, courseSlug string) (*domain.Progress, error)
	MarkDone(ctx context.Context, courseDir, courseSlug, taskSlug string) error
	MarkUndone(ctx context.Context, courseDir, courseSlug, taskSlug string) error
	Reset(ctx context.Context, courseDir string) error
}

type ProgressService struct {
	repo   progressRepository
	logger *zap.Logger
}

func NewProgressService(repo progressRepository, logger *zap.Logger) *ProgressService {
	return &ProgressService{
		repo:   repo,
		logger: logger,
	}
}

func (s *ProgressService) Load(ctx context.Context, courseDir, courseSlug string) (*domain.Progress, error) {
	progress, err := s.repo.Load(ctx, courseDir, courseSlug)
	if err != nil {
		s.logger.Error(
			"failed to load progress",
			zap.String("course_dir", courseDir),
			zap.String("course_slug", courseSlug),
			zap.Error(err),
		)

		return nil, fmt.Errorf("load progress: %w", err)
	}

	return progress, nil
}

func (s *ProgressService) MarkDone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	if err := s.repo.MarkDone(ctx, courseDir, courseSlug, taskSlug); err != nil {
		s.logger.Error(
			"failed to mark task done",
			zap.String("course_dir", courseDir),
			zap.String("course_slug", courseSlug),
			zap.String("task_slug", taskSlug),
			zap.Error(err),
		)

		return fmt.Errorf("mark task done: %w", err)
	}

	return nil
}

func (s *ProgressService) Reset(ctx context.Context, courseDir, courseSlug string) error {
	if err := s.repo.Reset(ctx, courseDir); err != nil {
		s.logger.Error(
			"failed to reset progress",
			zap.String("course_dir", courseDir),
			zap.String("course_slug", courseSlug),
			zap.Error(err),
		)

		return fmt.Errorf("reset progress: %w", err)
	}

	return nil
}

func (s *ProgressService) MarkUndone(ctx context.Context, courseDir, courseSlug, taskSlug string) error {
	if err := s.repo.MarkUndone(ctx, courseDir, courseSlug, taskSlug); err != nil {
		s.logger.Error(
			"failed to mark task undone",
			zap.String("course_dir", courseDir),
			zap.String("course_slug", courseSlug),
			zap.String("task_slug", taskSlug),
			zap.Error(err),
		)

		return fmt.Errorf("mark task undone: %w", err)
	}

	return nil
}
