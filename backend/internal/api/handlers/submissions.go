package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
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

	sub := &domain.Submission{
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

	id, err := h.submissionsService.Create(sub)
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

	subs, err := h.submissionsService.List(courseSlug, taskSlug)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load submissions")
		return
	}
	h.writeJSON(w, http.StatusOK, dto.ToSubmissionResponses(subs))
}
