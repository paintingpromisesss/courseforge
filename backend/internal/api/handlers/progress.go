package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
)

// @Summary Get course progress
// @Tags progress
// @Produce json
// @Param courseSlug path string true "Course slug"
// @Success 200 {object} dto.ProgressResp
// @Failure 404 {object} map[string]string
// @Router /progress/{courseSlug} [get]
func (h *Handler) getProgress(w http.ResponseWriter, r *http.Request) {
	courseSlug := chi.URLParam(r, "courseSlug")
	c := h.getCourseBySlug(courseSlug)
	if c == nil {
		h.writeError(w, http.StatusNotFound, "course not found")
		return
	}
	p, err := h.progress.Load(c.Dir, courseSlug)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load progress")
		return
	}
	h.writeJSON(w, http.StatusOK, dto.ToProgressResp(p))
}

// @Summary Mark task done or undone
// @Tags progress
// @Accept json
// @Param courseSlug path string true "Course slug"
// @Param taskSlug path string true "Task slug"
// @Param request body dto.ProgressUpdate true "Update"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /progress/{courseSlug}/tasks/{taskSlug} [put]
func (h *Handler) updateProgress(w http.ResponseWriter, r *http.Request) {
	courseSlug := chi.URLParam(r, "courseSlug")
	taskSlug := chi.URLParam(r, "taskSlug")

	c := h.getCourseBySlug(courseSlug)
	if c == nil {
		h.writeError(w, http.StatusNotFound, "course not found")
		return
	}

	var req dto.ProgressUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	if req.Done {
		err = h.progress.MarkDone(c.Dir, courseSlug, taskSlug)
	} else {
		err = h.progress.MarkUndone(c.Dir, courseSlug, taskSlug)
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to update progress")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
