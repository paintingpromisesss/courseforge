package service

import (
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"go.uber.org/zap"
)

type submissionRepository interface {
	Insert(sub *domain.Submission) (int64, error)
	List(courseSlug, taskSlug string) ([]domain.Submission, error)
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

func (s *SubmissionService) Create(sub *domain.Submission) (int64, error) {
	id, err := s.repo.Insert(sub)
	if err != nil {
		s.logger.Error("failed to insert submission", zap.Error(err))
		return 0, err
	}

	return id, nil
}

func (s *SubmissionService) List(courseSlug, taskSlug string) ([]domain.Submission, error) {
	submissions, err := s.repo.List(courseSlug, taskSlug)
	if err != nil {
		s.logger.Error(
			"failed to list submissions",
			zap.String("course_slug", courseSlug),
			zap.String("task_slug", taskSlug),
			zap.Error(err),
		)
		return nil, err
	}

	return submissions, nil
}
