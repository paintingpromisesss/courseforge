package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/application/service"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

type Handler struct {
	mu                 sync.RWMutex
	coursesDir         string
	runnersDir         string
	courses            map[string]*domain.Course
	runner             *runner.Runner
	progress           *service.ProgressService
	submissionsService *service.SubmissionService
	installJobs        sync.Map // presetID → *installProgress
}

func New(coursesDir, runnersDir string, courses map[string]*domain.Course, r *runner.Runner, ps *service.ProgressService, ss *service.SubmissionService) *Handler {
	return &Handler{
		coursesDir:         coursesDir,
		runnersDir:         runnersDir,
		courses:            courses,
		runner:             r,
		progress:           ps,
		submissionsService: ss,
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) getCourseBySlug(slug string) *domain.Course {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.courses[slug]
}

func (h *Handler) lookupUnit(w http.ResponseWriter, r *http.Request) (*domain.Course, *domain.Track, *domain.Topic, *domain.Unit, bool) {
	c := h.getCourseBySlug(chi.URLParam(r, "courseSlug"))
	if c == nil {
		h.writeError(w, http.StatusNotFound, "course not found")
		return nil, nil, nil, nil, false
	}
	track := c.FindTrack(chi.URLParam(r, "trackSlug"))
	if track == nil {
		h.writeError(w, http.StatusNotFound, "track not found")
		return nil, nil, nil, nil, false
	}
	topic := track.FindTopic(chi.URLParam(r, "topicSlug"))
	if topic == nil {
		h.writeError(w, http.StatusNotFound, "topic not found")
		return nil, nil, nil, nil, false
	}
	unit := topic.FindUnit(chi.URLParam(r, "unitSlug"))
	if unit == nil {
		h.writeError(w, http.StatusNotFound, "unit not found")
		return nil, nil, nil, nil, false
	}
	return c, track, topic, unit, true
}

func (h *Handler) lookupTask(w http.ResponseWriter, r *http.Request) (*domain.Course, *domain.Track, *domain.Topic, *domain.Unit, *domain.Task, bool) {
	c, track, topic, unit, ok := h.lookupUnit(w, r)
	if !ok {
		return nil, nil, nil, nil, nil, false
	}
	task := unit.FindTask(chi.URLParam(r, "taskSlug"))
	if task == nil {
		h.writeError(w, http.StatusNotFound, "task not found")
		return nil, nil, nil, nil, nil, false
	}
	return c, track, topic, unit, task, true
}
