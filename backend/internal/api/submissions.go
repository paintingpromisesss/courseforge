package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/submission"
)

type CreateSubmissionReq struct {
	CourseSlug  string `json:"course_slug"`
	TaskSlug    string `json:"task_slug"`
	Language    string `json:"language"`
	Code        string `json:"code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exit_code"`
	PassedTests int    `json:"passed_tests"`
	TotalTests  int    `json:"total_tests"`
	DurationMs  int64  `json:"duration_ms"`
	TimedOut    bool   `json:"timed_out"`
}

// @Summary Save a submission
// @Tags submissions
// @Accept json
// @Produce json
// @Param request body CreateSubmissionReq true "Submission"
// @Success 201 {object} submission.Submission
// @Failure 400 {object} map[string]string
// @Router /submissions [post]
func (h *Handler) createSubmission(w http.ResponseWriter, r *http.Request) {
	var req CreateSubmissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CourseSlug == "" || req.TaskSlug == "" || req.Language == "" || req.Code == "" {
		h.writeError(w, http.StatusBadRequest, "course_slug, task_slug, language and code are required")
		return
	}

	sub := &submission.Submission{
		CourseSlug:  req.CourseSlug,
		TaskSlug:    req.TaskSlug,
		Language:    req.Language,
		Code:        req.Code,
		Stdout:      req.Stdout,
		Stderr:      req.Stderr,
		ExitCode:    req.ExitCode,
		PassedTests: req.PassedTests,
		TotalTests:  req.TotalTests,
		DurationMs:  req.DurationMs,
		TimedOut:    req.TimedOut,
		CreatedAt:   time.Now().UTC(),
	}

	id, err := h.submissions.Insert(sub)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save submission")
		return
	}
	sub.ID = id
	h.writeJSON(w, http.StatusCreated, sub)
}

// @Summary List submissions for a task
// @Tags submissions
// @Produce json
// @Param courseSlug query string true "Course slug"
// @Param taskSlug   query string true "Task slug"
// @Success 200 {array} submission.Submission
// @Failure 400 {object} map[string]string
// @Router /submissions [get]
func (h *Handler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	courseSlug := r.URL.Query().Get("courseSlug")
	taskSlug := r.URL.Query().Get("taskSlug")
	if courseSlug == "" || taskSlug == "" {
		h.writeError(w, http.StatusBadRequest, "courseSlug and taskSlug query params required")
		return
	}

	subs, err := h.submissions.List(courseSlug, taskSlug)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load submissions")
		return
	}
	if subs == nil {
		subs = []submission.Submission{}
	}
	h.writeJSON(w, http.StatusOK, subs)
}
