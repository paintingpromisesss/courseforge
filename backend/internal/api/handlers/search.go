package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/application/service"
)

const (
	defaultSearchLimit = 50
	snippetCacheTTL    = 30 * time.Second // fallback staleness bound if mtime is unavailable
)

// searchStatementCache caches statement.md text keyed by task path,
// invalidated via file mtime (covers on-disk edits from MCP tools).
type searchStatementCache struct {
	mu    sync.Mutex
	ents  map[string]searchCacheEntry
	files map[string]int64 // reusable mtime snapshot to avoid double-stat cost per query
}

type searchCacheEntry struct {
	modTime time.Time
	text    string
	checked time.Time
}

func newSearchStatementCache() *searchStatementCache {
	return &searchStatementCache{ents: make(map[string]searchCacheEntry)}
}

func (c *searchStatementCache) get(path string) (string, bool) {
	c.mu.Lock()
	ent, ok := c.ents[path]
	if ok && time.Since(ent.checked) < time.Second {
		c.mu.Unlock()
		return ent.text, true
	}
	c.mu.Unlock()

	st, err := os.Stat(path)
	if err != nil {
		c.mu.Lock()
		delete(c.ents, path)
		c.mu.Unlock()
		return "", false
	}
	if ok && st.ModTime().Equal(ent.modTime) {
		c.mu.Lock()
		ent.checked = time.Now()
		c.ents[path] = ent
		c.mu.Unlock()
		return ent.text, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	c.mu.Lock()
	c.ents[path] = searchCacheEntry{modTime: st.ModTime(), text: string(data), checked: time.Now()}
	c.mu.Unlock()
	return string(data), true
}

// searchStatementReader returns a lazily-evaluated content provider bound to
// the current course snapshot. It must be called while the courses snapshot
// (SearchTask values) is already copied out — it re-derives the file path
// from the copied item, so it is safe to keep after RUnlock.
func (h *Handler) searchStatementReader() func(service.SearchTask) (string, bool) {
	return func(t service.SearchTask) (string, bool) {
		if t.Statement == "" {
			return "", false
		}
		path := filepath.Join(h.coursesDir, t.CourseDir, t.Track, t.Topic, t.Unit, t.Task, t.Statement)
		return h.searchCache.get(path)
	}
}

// snapshotSearchTasks copies the flat search index under the courses lock.
func (h *Handler) snapshotSearchTasks() []service.SearchTask {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return service.FlattenSearchTasks(h.courses)
}

// @Summary Search tasks by words, tags and statement content
// @Tags search
// @Produce json
// @Param q query string true "Query; space-separated words, substring match, AND semantics"
// @Param limit query int false "Max results (default 50)"
// @Param course query string false "Restrict search to one course slug"
// @Success 200 {object} dto.SearchResp
// @Failure 400 {object} map[string]string
// @Router /search/tasks [get]
func (h *Handler) searchTasks(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		h.writeError(w, http.StatusBadRequest, "query must be at least 2 characters")
		return
	}
	limit := defaultSearchLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	courseFilter := r.URL.Query().Get("course")

	tasks := h.snapshotSearchTasks()
	if courseFilter != "" {
		filtered := make([]service.SearchTask, 0, len(tasks))
		for _, t := range tasks {
			if t.Course == courseFilter {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	results := service.SearchTasks(tasks, q, limit, h.searchStatementReader())
	h.writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// @Summary Flat task catalog for AI-assisted search
// @Tags search
// @Produce json
// @Success 200 {object} dto.SearchCatalogResp
// @Router /search/catalog [get]
func (h *Handler) searchCatalog(w http.ResponseWriter, r *http.Request) {
	tasks := h.snapshotSearchTasks()
	h.writeJSON(w, http.StatusOK, map[string]any{"tasks": service.BuildCatalog(tasks)})
}
