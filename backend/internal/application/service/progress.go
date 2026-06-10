package service

import (
	"errors"
	"fmt"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"go.uber.org/zap"
)

type progressRepository interface {
	Load(courseDir, courseSlug string) (*domain.Progress, error)
	MarkDone(courseDir, courseSlug, taskSlug string) error
	MarkUndone(courseDir, courseSlug, taskSlug string) error
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

func (s *ProgressService) Load(courseDir, courseSlug string) (*domain.Progress, error) {
	if courseDir == "" {
		return nil, errors.New("course dir is required")
	}

	if courseSlug == "" {
		return nil, errors.New("course slug is required")
	}

	progress, err := s.repo.Load(courseDir, courseSlug)
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

func (s *ProgressService) MarkDone(courseDir, courseSlug, taskSlug string) error {
	if courseDir == "" {
		return errors.New("course dir is required")
	}

	if courseSlug == "" {
		return errors.New("course slug is required")
	}

	if taskSlug == "" {
		return errors.New("task slug is required")
	}

	if err := s.repo.MarkDone(courseDir, courseSlug, taskSlug); err != nil {
		s.logger.Error(
			"failed to mark task as done",
			zap.String("course_dir", courseDir),
			zap.String("course_slug", courseSlug),
			zap.String("task_slug", taskSlug),
			zap.Error(err),
		)

		return fmt.Errorf("mark task done: %w", err)
	}

	return nil
}

func (s *ProgressService) MarkUndone(courseDir, courseSlug, taskSlug string) error {
	if courseDir == "" {
		return errors.New("course dir is required")
	}

	if courseSlug == "" {
		return errors.New("course slug is required")
	}

	if taskSlug == "" {
		return errors.New("task slug is required")
	}

	if err := s.repo.MarkUndone(courseDir, courseSlug, taskSlug); err != nil {
		s.logger.Error(
			"failed to mark task as undone",
			zap.String("course_dir", courseDir),
			zap.String("course_slug", courseSlug),
			zap.String("task_slug", taskSlug),
			zap.Error(err),
		)

		return fmt.Errorf("mark task undone: %w", err)
	}

	return nil
}
