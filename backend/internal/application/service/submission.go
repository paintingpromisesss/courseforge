package service

import (
	"context"
	"fmt"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"go.uber.org/zap"
)

type submissionRepository interface {
	Insert(ctx context.Context, sub *domain.Submission) (int64, error)
	List(ctx context.Context, courseSlug, taskSlug string) ([]domain.Submission, error)
}

type SubmissionService struct {
	repo   submissionRepository
	logger *zap.Logger
}

func NewSubmissionService(repo submissionRepository, logger *zap.Logger) *SubmissionService {
	return &SubmissionService{
		repo:   repo,
		logger: logger,
	}
}

func (s *SubmissionService) Create(ctx context.Context, sub *domain.Submission) (int64, error) {
	id, err := s.repo.Insert(ctx, sub)
	if err != nil {
		s.logger.Error("failed to insert submission", zap.Error(err))
		return 0, fmt.Errorf("create submission: %w", err)
	}

	return id, nil
}

func (s *SubmissionService) List(ctx context.Context, courseSlug, taskSlug string) ([]domain.Submission, error) {
	submissions, err := s.repo.List(ctx, courseSlug, taskSlug)
	if err != nil {
		s.logger.Error(
			"failed to list submissions",
			zap.String("course_slug", courseSlug),
			zap.String("task_slug", taskSlug),
			zap.Error(err),
		)
		return nil, fmt.Errorf("list submissions: %w", err)
	}

	return submissions, nil
}
