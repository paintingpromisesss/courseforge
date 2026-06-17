package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	courseparser "github.com/paintingpromisesss/courseforge/internal/infrastructure/parser/course"
	"gopkg.in/yaml.v3"
)

// catalogManifest is the on-disk catalog.yaml shape (reference membership).
type catalogManifest struct {
	Slug        string   `yaml:"slug"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	Courses     []string `yaml:"courses"`
}

func marshalCatalogManifest(slug, title, description string, courses []string) ([]byte, error) {
	if courses == nil {
		courses = []string{}
	}
	return yaml.Marshal(catalogManifest{slug, title, description, courses})
}

var slugSanitize = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a title into a filesystem-safe slug.
func slugify(title string) string {
	s := slugSanitize.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "group"
	}
	return s
}

// uniqueSlug returns base, or base-2, base-3, … so it collides with no existing
// course, catalog, or on-disk directory. Caller must hold no lock.
func (h *Handler) uniqueSlug(base string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.uniqueSlugLocked(base)
}

// uniqueSlugLocked is uniqueSlug for callers already holding h.mu.
func (h *Handler) uniqueSlugLocked(base string) string {
	taken := func(s string) bool {
		if _, ok := h.courses[s]; ok {
			return true
		}
		if _, ok := h.catalogs[s]; ok {
			return true
		}
		_, err := os.Stat(filepath.Join(h.coursesDir, s))
		return err == nil
	}
	slug := base
	for i := 2; taken(slug); i++ {
		slug = base + "-" + strconv.Itoa(i)
	}
	return slug
}

// writeCatalogManifest persists a registered catalog's catalog.yaml to disk.
// Caller must hold h.mu.
func (h *Handler) writeCatalogManifest(slug string) error {
	cat := h.catalogs[slug]
	data, err := marshalCatalogManifest(cat.Slug, cat.Title, cat.Description, cat.CourseSlugs)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(h.coursesDir, cat.Dir, "catalog.yaml"), data, 0644)
}

// @Summary Create an empty catalog (group)
// @Tags courses
// @Accept json
// @Produce json
// @Param body body dto.CreateCatalogReq true "Title and optional description"
// @Success 201 {object} dto.SlugResp
// @Failure 400 {object} map[string]string
// @Router /catalogs [post]
func (h *Handler) createCatalog(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCatalogReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		h.writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	slug := h.uniqueSlug(slugify(req.Title))
	dir := filepath.Join(h.coursesDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create catalog dir")
		return
	}

	data, err := marshalCatalogManifest(slug, req.Title, strings.TrimSpace(req.Description), nil)
	if err != nil {
		_ = os.RemoveAll(dir)
		h.writeError(w, http.StatusInternalServerError, "failed to build catalog manifest")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.yaml"), data, 0644); err != nil {
		_ = os.RemoveAll(dir)
		h.writeError(w, http.StatusInternalServerError, "failed to write catalog manifest")
		return
	}

	if _, err := h.loadAndRegisterCatalog(slug); err != nil {
		_ = os.RemoveAll(dir)
		h.writeError(w, http.StatusInternalServerError, "failed to register catalog: "+err.Error())
		return
	}
	h.writeJSON(w, http.StatusCreated, dto.SlugResp{Slug: slug})
}

// @Summary Update a catalog's title, description, or course membership
// @Tags courses
// @Accept json
// @Param catalogSlug path string true "Catalog slug"
// @Param body body dto.PatchCatalogReq true "Fields to update"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /catalogs/{catalogSlug} [patch]
func (h *Handler) patchCatalog(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "catalogSlug")

	var req dto.PatchCatalogReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.mu.Lock()
	cat := h.catalogs[slug]
	if cat == nil {
		h.mu.Unlock()
		h.writeError(w, http.StatusNotFound, "catalog not found")
		return
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			h.mu.Unlock()
			h.writeError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		cat.Title = t
	}
	if req.Description != nil {
		cat.Description = strings.TrimSpace(*req.Description)
	}
	if req.Courses != nil {
		for _, cs := range *req.Courses {
			if h.courses[cs] == nil {
				h.mu.Unlock()
				h.writeError(w, http.StatusBadRequest, "unknown course: "+cs)
				return
			}
		}
		cat.CourseSlugs = dedupe(*req.Courses)
		courseparser.ResolveCatalogCourses(cat, h.courses)
	}
	err := h.writeCatalogManifest(slug)
	h.mu.Unlock()

	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to persist catalog")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// dedupeImportedCatalog renames any nested course in a freshly imported catalog
// whose slug already belongs to a registered course, so importing a catalog never
// clobbers existing standalone courses that happen to share a slug. It updates the
// course folder name, the course.yaml slug field, and the catalog.yaml reference.
func (h *Handler) dedupeImportedCatalog(dirSlug string) error {
	cat, err := courseparser.LoadCatalogManifest(h.coursesDir, dirSlug)
	if err != nil {
		return err
	}
	catDir := filepath.Join(h.coursesDir, dirSlug)

	h.mu.RLock()
	registered := func(s string) bool { _, ok := h.courses[s]; return ok }
	h.mu.RUnlock()

	assigned := map[string]bool{}
	newSlugs := make([]string, len(cat.CourseSlugs))
	changed := false
	for i, cs := range cat.CourseSlugs {
		newSlugs[i] = cs
		courseDir := filepath.Join(catDir, cs)
		if _, err := os.Stat(filepath.Join(courseDir, "course.yaml")); err != nil {
			continue // reference to an external course, not a nested folder
		}
		if !registered(cs) && !assigned[cs] {
			assigned[cs] = true
			continue
		}
		ns := cs
		for i := 2; registered(ns) || assigned[ns]; i++ {
			ns = cs + "-" + strconv.Itoa(i)
		}
		if err := os.Rename(courseDir, filepath.Join(catDir, ns)); err != nil {
			return err
		}
		if err := rewriteCourseSlug(filepath.Join(catDir, ns, "course.yaml"), ns); err != nil {
			return err
		}
		newSlugs[i] = ns
		assigned[ns] = true
		changed = true
	}

	if !changed {
		return nil
	}
	data, err := marshalCatalogManifest(cat.Slug, cat.Title, cat.Description, newSlugs)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(catDir, "catalog.yaml"), data, 0644)
}

var courseSlugLine = regexp.MustCompile(`(?m)^slug:.*$`)

func rewriteCourseSlug(path, newSlug string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(courseSlugLine.ReplaceAllString(string(data), "slug: "+newSlug)), 0644)
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
