package handlers

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
)

// @Summary List all courses
// @Tags courses
// @Produce json
// @Success 200 {array} dto.CourseItem
// @Router /courses [get]
func (h *Handler) listCourses(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	items := make([]dto.CourseItem, 0, len(h.courses))
	for _, c := range h.courses {
		items = append(items, dto.ToCourseItem(c))
	}
	h.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
	h.writeJSON(w, http.StatusOK, items)
}

// @Summary Get full course tree
// @Tags courses
// @Produce json
// @Param courseSlug path string true "Course slug"
// @Success 200 {object} dto.CourseDetail
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug} [get]
func (h *Handler) getCourse(w http.ResponseWriter, r *http.Request) {
	c := h.getCourseBySlug(chi.URLParam(r, "courseSlug"))
	if c == nil {
		h.writeError(w, http.StatusNotFound, "course not found")
		return
	}
	h.writeJSON(w, http.StatusOK, dto.ToCourseDetail(c))
}

// @Summary Get theory file (markdown)
// @Tags content
// @Produce plain
// @Param courseSlug path string true "Course slug"
// @Param trackSlug path string true "Track slug"
// @Param topicSlug path string true "Topic slug"
// @Param unitSlug path string true "Unit slug"
// @Success 200 {string} string
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/theory [get]
func (h *Handler) getTheory(w http.ResponseWriter, r *http.Request) {
	c, track, topic, unit, ok := h.lookupUnit(w, r)
	if !ok {
		return
	}
	if unit.Theory == "" {
		h.writeError(w, http.StatusNotFound, "unit has no theory")
		return
	}
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, unit.Theory)
	serveTextFile(w, path)
}

// @Summary Get task statement (markdown)
// @Tags content
// @Produce plain
// @Param courseSlug path string true "Course slug"
// @Param trackSlug path string true "Track slug"
// @Param topicSlug path string true "Topic slug"
// @Param unitSlug path string true "Unit slug"
// @Param taskSlug path string true "Task slug"
// @Success 200 {string} string
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/statement [get]
func (h *Handler) getStatement(w http.ResponseWriter, r *http.Request) {
	c, track, topic, unit, task, ok := h.lookupTask(w, r)
	if !ok {
		return
	}
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, task.Statement)
	serveTextFile(w, path)
}

// @Summary Get task code template
// @Tags content
// @Produce plain
// @Param courseSlug path string true "Course slug"
// @Param trackSlug path string true "Track slug"
// @Param topicSlug path string true "Topic slug"
// @Param unitSlug path string true "Unit slug"
// @Param taskSlug path string true "Task slug"
// @Param lang query string true "Language (e.g. go, python)"
// @Success 200 {string} string
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/template [get]
func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	c, track, topic, unit, task, ok := h.lookupTask(w, r)
	if !ok {
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		h.writeError(w, http.StatusBadRequest, "lang query param required")
		return
	}
	ld, exists := task.Languages[lang]
	if !exists {
		h.writeError(w, http.StatusNotFound, "language not available for this task")
		return
	}
	filename := ld.Template
	if solution := r.URL.Query().Get("solution"); solution == "1" || strings.EqualFold(solution, "true") {
		if ld.Solution == "" {
			h.writeError(w, http.StatusNotFound, "no solution for this language")
			return
		}
		filename = ld.Solution
	}
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, lang, filename)
	serveTextFile(w, path)
}

// @Summary Get task test file
// @Tags content
// @Produce plain
// @Param courseSlug path string true "Course slug"
// @Param trackSlug path string true "Track slug"
// @Param topicSlug path string true "Topic slug"
// @Param unitSlug path string true "Unit slug"
// @Param taskSlug path string true "Task slug"
// @Param lang query string true "Language (e.g. go, python)"
// @Success 200 {string} string
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/tests [get]
func (h *Handler) getTests(w http.ResponseWriter, r *http.Request) {
	c, track, topic, unit, task, ok := h.lookupTask(w, r)
	if !ok {
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		h.writeError(w, http.StatusBadRequest, "lang query param required")
		return
	}
	ld, exists := task.Languages[lang]
	if !exists {
		h.writeError(w, http.StatusNotFound, "language not available for this task")
		return
	}
	if ld.Tests == "" {
		h.writeError(w, http.StatusNotFound, "no tests for this language")
		return
	}
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, lang, ld.Tests)
	serveTextFile(w, path)
}

// @Summary Get unit asset (image/svg/etc)
// @Tags content
// @Produce octet-stream
// @Param courseSlug path string true "Course slug"
// @Param trackSlug path string true "Track slug"
// @Param topicSlug path string true "Topic slug"
// @Param unitSlug path string true "Unit slug"
// @Param filename path string true "Asset filename"
// @Success 200
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/assets/{filename} [get]
func (h *Handler) getUnitAsset(w http.ResponseWriter, r *http.Request) {
	c, track, topic, unit, ok := h.lookupUnit(w, r)
	if !ok {
		return
	}
	filename := filepath.Base(chi.URLParam(r, "filename"))
	if strings.Contains(filename, "..") {
		h.writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, "assets", filename)
	http.ServeFile(w, r, path)
}

// @Summary Get task asset (image/svg/etc)
// @Tags content
// @Produce octet-stream
// @Param courseSlug path string true "Course slug"
// @Param trackSlug path string true "Track slug"
// @Param topicSlug path string true "Topic slug"
// @Param unitSlug path string true "Unit slug"
// @Param taskSlug path string true "Task slug"
// @Param filename path string true "Asset filename"
// @Success 200
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug}/tracks/{trackSlug}/topics/{topicSlug}/units/{unitSlug}/tasks/{taskSlug}/assets/{filename} [get]
func (h *Handler) getTaskAsset(w http.ResponseWriter, r *http.Request) {
	c, track, topic, unit, task, ok := h.lookupTask(w, r)
	if !ok {
		return
	}
	filename := filepath.Base(chi.URLParam(r, "filename"))
	if strings.Contains(filename, "..") {
		h.writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, "assets", filename)
	http.ServeFile(w, r, path)
}
