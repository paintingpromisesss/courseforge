package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// @Summary Save a submission
// @Tags submissions
// @Accept json
// @Produce json
// @Param request body dto.CreateSubmissionReq true "Submission"
// @Success 201 {object} dto.SubmissionResp
// @Failure 400 {object} map[string]string
// @Router /submissions [post]
func (h *Handler) createSubmission(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSubmissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CourseSlug == "" || req.TaskSlug == "" || req.Language == "" || req.Code == "" {
		h.writeError(w, http.StatusBadRequest, "course_slug, task_slug, language and code are required")
		return
	}

	h.mu.RLock()
	c, ok := h.courses[req.CourseSlug]
	h.mu.RUnlock()
	if !ok {
		h.writeError(w, http.StatusNotFound, "course not found")
		return
	}
	track, topic, unit, task := c.FindTaskPath(req.TaskSlug)
	if task == nil {
		h.writeError(w, http.StatusNotFound, "task not found")
		return
	}
	ld, exists := task.Languages[req.Language]
	if !exists {
		h.writeError(w, http.StatusNotFound, "language not available for this task")
		return
	}
	if ld.Tests == "" {
		h.writeError(w, http.StatusBadRequest, "no tests configured for this language")
		return
	}
	testPath := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, req.Language, ld.Tests)
	testCode, err := os.ReadFile(testPath)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load task tests")
		return
	}

	var schemaContent string
	if ld.Schema != "" {
		schemaPath := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, req.Language, ld.Schema)
		schemaBytes, err := os.ReadFile(schemaPath)
		if err == nil {
			schemaContent = string(schemaBytes)
		}
	}

	result, err := h.runner.Run(r.Context(), runner.RunRequest{
		Language: req.Language,
		Code:     req.Code,
		TestCode: string(testCode),
		Schema:   schemaContent,
	})
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	passed, total := runner.CountTestResults(req.Language, result.Stdout, result.Stderr)

	sub := &domain.Submission{
		CourseSlug:  req.CourseSlug,
		TaskSlug:    req.TaskSlug,
		Language:    req.Language,
		Code:        req.Code,
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		PassedTests: passed,
		TotalTests:  total,
		DurationMs:  result.Duration.Milliseconds(),
		TimedOut:    result.TimedOut,
		CreatedAt:   time.Now().UTC(),
	}

	id, err := h.submissions.Create(r.Context(), sub)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save submission")
		return
	}
	sub.ID = id
	h.writeJSON(w, http.StatusCreated, dto.ToSubmissionResp(*sub))
}

// @Summary List submissions for a task
// @Tags submissions
// @Produce json
// @Param courseSlug query string true "Course slug"
// @Param taskSlug   query string true "Task slug"
// @Success 200 {array} dto.SubmissionResp
// @Failure 400 {object} map[string]string
// @Router /submissions [get]
func (h *Handler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	courseSlug := r.URL.Query().Get("courseSlug")
	taskSlug := r.URL.Query().Get("taskSlug")
	if courseSlug == "" || taskSlug == "" {
		h.writeError(w, http.StatusBadRequest, "courseSlug and taskSlug query params required")
		return
	}

	subs, err := h.submissions.List(r.Context(), courseSlug, taskSlug)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load submissions")
		return
	}
	h.writeJSON(w, http.StatusOK, dto.ToSubmissionResponses(subs))
}
