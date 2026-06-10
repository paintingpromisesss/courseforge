package domain

import "time"

type Submission struct {
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
