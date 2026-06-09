package api

import (
	"net/http"
	"path/filepath"
	"sort"

	"github.com/go-chi/chi/v5"
)

type CourseItem struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Language    string `json:"language"`
}

type CourseDetail struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Language    string      `json:"language"`
	Tracks      []TrackItem `json:"tracks"`
}

type TrackItem struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Topics      []TopicItem `json:"topics"`
}

type TopicItem struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Units       []UnitItem `json:"units"`
}

type UnitItem struct {
	Slug      string     `json:"slug"`
	Title     string     `json:"title"`
	HasTheory bool       `json:"has_theory"`
	Tasks     []TaskItem `json:"tasks"`
}

type TaskItem struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Languages []string `json:"languages"`
}

// @Summary List all courses
// @Tags courses
// @Produce json
// @Success 200 {array} CourseItem
// @Router /courses [get]
func (h *Handler) listCourses(w http.ResponseWriter, r *http.Request) {
	items := make([]CourseItem, 0, len(h.courses))
	for _, c := range h.courses {
		items = append(items, CourseItem{
			Slug: c.Slug, Title: c.Title,
			Description: c.Description, Language: c.Language,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
	h.writeJSON(w, http.StatusOK, items)
}

// @Summary Get full course tree
// @Tags courses
// @Produce json
// @Param courseSlug path string true "Course slug"
// @Success 200 {object} CourseDetail
// @Failure 404 {object} map[string]string
// @Router /courses/{courseSlug} [get]
func (h *Handler) getCourse(w http.ResponseWriter, r *http.Request) {
	c := h.courses[chi.URLParam(r, "courseSlug")]
	if c == nil {
		h.writeError(w, http.StatusNotFound, "course not found")
		return
	}
	h.writeJSON(w, http.StatusOK, toCourseDetail(c))
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
	path := filepath.Join(h.coursesDir, c.Dir, track.Slug, topic.Slug, unit.Slug, task.Slug, lang, ld.Template)
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
