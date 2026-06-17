package dto

import (
	"time"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type SlugResp struct {
	Slug string `json:"slug"`
}

type DetectResp struct {
	Status  string `json:"status"` // ok | broken | missing
	Binary  string `json:"binary"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

type ProgressResp struct {
	CourseSlug     string          `json:"course_slug"`
	CompletedTasks map[string]bool `json:"completed_tasks"`
}

type SubmissionResp struct {
	ID          int64     `json:"id"`
	CourseSlug  string    `json:"course_slug"`
	TaskSlug    string    `json:"task_slug"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	ExitCode    int       `json:"exit_code"`
	PassedTests int       `json:"passed_tests"`
	TotalTests  int       `json:"total_tests"`
	DurationMs  int64     `json:"duration_ms"`
	TimedOut    bool      `json:"timed_out"`
	CreatedAt   time.Time `json:"created_at"`
}

func ToProgressResp(p *domain.Progress) ProgressResp {
	if p == nil {
		return ProgressResp{CompletedTasks: map[string]bool{}}
	}

	return ProgressResp{
		CourseSlug:     p.CourseSlug,
		CompletedTasks: cloneBoolMap(p.CompletedTasks),
	}
}

func ToSubmissionResp(sub domain.Submission) SubmissionResp {
	return SubmissionResp{
		ID:          sub.ID,
		CourseSlug:  sub.CourseSlug,
		TaskSlug:    sub.TaskSlug,
		Language:    sub.Language,
		Code:        sub.Code,
		Stdout:      sub.Stdout,
		Stderr:      sub.Stderr,
		ExitCode:    sub.ExitCode,
		PassedTests: sub.PassedTests,
		TotalTests:  sub.TotalTests,
		DurationMs:  sub.DurationMs,
		TimedOut:    sub.TimedOut,
		CreatedAt:   sub.CreatedAt,
	}
}

func ToSubmissionResponses(subs []domain.Submission) []SubmissionResp {
	if len(subs) == 0 {
		return []SubmissionResp{}
	}

	out := make([]SubmissionResp, len(subs))
	for i, sub := range subs {
		out[i] = ToSubmissionResp(sub)
	}

	return out
}

func cloneBoolMap(src map[string]bool) map[string]bool {
	if src == nil {
		return map[string]bool{}
	}

	dst := make(map[string]bool, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}
