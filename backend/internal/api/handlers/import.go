package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	courseparser "github.com/paintingpromisesss/courseforge/internal/infrastructure/parser/course"
)

// @Summary Upload a course or catalog folder
// @Description Multipart upload from a webkitdirectory input. Send a "paths" field (JSON array of relative paths, in file order) followed by the "files" parts.
// @Tags courses
// @Accept multipart/form-data
// @Produce json
// @Param paths formData string true "JSON array of relative file paths, in file order"
// @Param files formData file true "File contents (one part per file)"
// @Success 201 {object} dto.SlugResp
// @Failure 400 {object} map[string]string
// @Router /courses/upload [post]
func (h *Handler) uploadCourse(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	tmpDir, err := os.MkdirTemp("", "cf-import-*")
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	var relPaths []string
	rootDir := ""
	fileIdx := 0

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "failed to read upload")
			return
		}

		switch part.FormName() {
		case "paths":
			data, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				h.writeError(w, http.StatusBadRequest, "failed to read paths")
				return
			}
			if err := json.Unmarshal(data, &relPaths); err != nil {
				h.writeError(w, http.StatusBadRequest, "invalid paths field")
				return
			}

		case "files":
			if fileIdx >= len(relPaths) {
				part.Close()
				h.writeError(w, http.StatusBadRequest, "files received before 'paths' field or count mismatch")
				return
			}
			rel := strings.TrimPrefix(filepath.ToSlash(relPaths[fileIdx]), "./")
			fileIdx++

			if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
				part.Close()
				h.writeError(w, http.StatusBadRequest, "invalid file path: "+rel)
				return
			}
			parts := strings.SplitN(rel, "/", 2)
			if len(parts) < 2 {
				part.Close()
				h.writeError(w, http.StatusBadRequest, "expected files with relative paths (use webkitdirectory input)")
				return
			}
			if rootDir == "" {
				rootDir = parts[0]
			} else if parts[0] != rootDir {
				part.Close()
				h.writeError(w, http.StatusBadRequest, "all uploaded files must come from the same root folder")
				return
			}

			target := filepath.Join(tmpDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				part.Close()
				h.writeError(w, http.StatusInternalServerError, "failed to create directory")
				return
			}
			dst, err := os.Create(target)
			if err != nil {
				part.Close()
				h.writeError(w, http.StatusInternalServerError, "failed to write file")
				return
			}
			_, err = io.Copy(dst, part)
			dst.Close()
			part.Close()
			if err != nil {
				h.writeError(w, http.StatusInternalServerError, "failed to save file")
				return
			}

		default:
			part.Close()
		}
	}

	if rootDir == "" {
		h.writeError(w, http.StatusBadRequest, "no files provided")
		return
	}

	sourceDir := filepath.Join(tmpDir, rootDir)
	slug, herr := h.importFromDir(sourceDir)
	if herr != nil {
		h.writeError(w, herr.status, herr.msg)
		return
	}
	h.writeJSON(w, http.StatusCreated, dto.SlugResp{Slug: slug})
}

// httpError carries an HTTP status alongside a client-facing message.
type httpError struct {
	status int
	msg    string
}

// importFromDir inspects sourceDir (course or catalog), moves it into coursesDir, and registers it.
func (h *Handler) importFromDir(sourceDir string) (string, *httpError) {
	hasCatalog := fileExists(filepath.Join(sourceDir, "catalog.yaml"))
	hasCourse := fileExists(filepath.Join(sourceDir, "course.yaml"))

	switch {
	case hasCatalog:
		return h.importCatalogDir(sourceDir)
	case hasCourse:
		return h.importCourseDir(sourceDir)
	default:
		return "", &httpError{http.StatusBadRequest, "neither course.yaml nor catalog.yaml found in folder"}
	}
}

func (h *Handler) importCourseDir(sourceDir string) (string, *httpError) {
	c, err := courseparser.LoadOne(filepath.Dir(sourceDir), filepath.Base(sourceDir))
	if err != nil {
		return "", &httpError{http.StatusBadRequest, "invalid course: " + err.Error()}
	}
	if h.getCourseBySlug(c.Slug) != nil {
		return "", &httpError{http.StatusConflict, fmt.Sprintf("course slug %q already exists", c.Slug)}
	}
	destDir := filepath.Join(h.coursesDir, c.Slug)
	if _, err := os.Stat(destDir); err == nil {
		return "", &httpError{http.StatusConflict, fmt.Sprintf("course slug %q already exists on disk", c.Slug)}
	}
	if err := os.Rename(sourceDir, destDir); err != nil {
		return "", &httpError{http.StatusInternalServerError, "failed to install course"}
	}
	registered, err := h.loadAndRegisterCourse(c.Slug)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return "", &httpError{http.StatusBadRequest, "invalid course: " + err.Error()}
	}
	return registered.Slug, nil
}

func (h *Handler) importCatalogDir(sourceDir string) (string, *httpError) {
	cat, err := courseparser.LoadCatalogOne(filepath.Dir(sourceDir), filepath.Base(sourceDir))
	if err != nil {
		return "", &httpError{http.StatusBadRequest, "invalid catalog: " + err.Error()}
	}
	destDir := filepath.Join(h.coursesDir, cat.Slug)
	if _, err := os.Stat(destDir); err == nil {
		return "", &httpError{http.StatusConflict, fmt.Sprintf("catalog slug %q already exists on disk", cat.Slug)}
	}
	if err := os.Rename(sourceDir, destDir); err != nil {
		return "", &httpError{http.StatusInternalServerError, "failed to install catalog"}
	}
	// rename any nested course whose slug collides with an existing course, so the
	// import never hides standalone courses that happen to share a slug.
	if err := h.dedupeImportedCatalog(cat.Slug); err != nil {
		_ = os.RemoveAll(destDir)
		return "", &httpError{http.StatusInternalServerError, "failed to import catalog: " + err.Error()}
	}
	registered, err := h.loadAndRegisterCatalog(cat.Slug)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return "", &httpError{http.StatusBadRequest, "invalid catalog: " + err.Error()}
	}
	return registered.Slug, nil
}

// @Summary Delete a standalone course
// @Tags courses
// @Produce json
// @Param courseSlug path string true "Course slug"
// @Success 204
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /courses/{courseSlug} [delete]
func (h *Handler) deleteCourse(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "courseSlug")

	h.mu.Lock()
	c := h.courses[slug]
	if c == nil {
		h.mu.Unlock()
		h.writeError(w, http.StatusNotFound, "course not found")
		return
	}
	if strings.Contains(c.Dir, "/") {
		h.mu.Unlock()
		h.writeError(w, http.StatusConflict, "course belongs to a catalog; delete the catalog instead")
		return
	}
	dir := filepath.Join(h.coursesDir, c.Dir)
	delete(h.courses, slug)
	// drop the course from any catalog that references it, so manifests stay consistent
	for catSlug, cat := range h.catalogs {
		if !containsString(cat.CourseSlugs, slug) {
			continue
		}
		cat.CourseSlugs = removeString(cat.CourseSlugs, slug)
		courseparser.ResolveCatalogCourses(cat, h.courses)
		_ = h.writeCatalogManifest(catSlug)
	}
	h.mu.Unlock()

	if err := os.RemoveAll(dir); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to delete course files")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func removeString(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// @Summary Delete a catalog, optionally deleting its courses
// @Description By default member courses are kept (legacy nested courses are moved
// @Description out to top-level). With purge=1 every member course is deleted physically.
// @Tags courses
// @Produce json
// @Param catalogSlug path string true "Catalog slug"
// @Param purge query string false "1/true to also delete member courses physically"
// @Success 204
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /catalogs/{catalogSlug} [delete]
func (h *Handler) deleteCatalog(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "catalogSlug")
	purge := queryBool(r, "purge")

	h.mu.Lock()
	cat := h.catalogs[slug]
	if cat == nil {
		h.mu.Unlock()
		h.writeError(w, http.StatusNotFound, "catalog not found")
		return
	}
	catDir := filepath.Join(h.coursesDir, cat.Dir)
	members := append([]string(nil), cat.CourseSlugs...)
	dirPrefix := cat.Dir + "/"
	delete(h.catalogs, slug)

	var opErr error
	if purge {
		// delete every member course: top-level dirs are removed here, nested ones
		// go away with the catalog dir; in both cases unregister + drop from other catalogs.
		for _, cs := range members {
			if err := h.removeCourseLocked(cs); err != nil && opErr == nil {
				opErr = err
			}
		}
	} else {
		// keep courses: move legacy-nested ones out to a top-level folder so they
		// survive removal of the catalog dir. Reference courses are already top-level.
		for _, cs := range members {
			c := h.courses[cs]
			if c == nil || !strings.HasPrefix(c.Dir, dirPrefix) {
				continue
			}
			// slug is globally unique, so its own folder name is free at top level;
			// keep slug == folder name so course.yaml stays valid.
			dest := c.Slug
			if err := os.Rename(filepath.Join(h.coursesDir, c.Dir), filepath.Join(h.coursesDir, dest)); err != nil {
				if opErr == nil {
					opErr = err
				}
				continue
			}
			c.Dir = dest
		}
	}
	h.mu.Unlock()

	if opErr != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to delete catalog: "+opErr.Error())
		return
	}
	if err := os.RemoveAll(catDir); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to delete catalog files")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeCourseLocked unregisters a course, strips it from every catalog that
// references it, and deletes its files when it lives in a top-level folder
// (nested courses are removed together with their catalog dir). Caller holds h.mu.
func (h *Handler) removeCourseLocked(slug string) error {
	c := h.courses[slug]
	if c == nil {
		return nil
	}
	delete(h.courses, slug)
	for catSlug, cat := range h.catalogs {
		if !containsString(cat.CourseSlugs, slug) {
			continue
		}
		cat.CourseSlugs = removeString(cat.CourseSlugs, slug)
		courseparser.ResolveCatalogCourses(cat, h.courses)
		_ = h.writeCatalogManifest(catSlug)
	}
	if !strings.Contains(c.Dir, "/") {
		return os.RemoveAll(filepath.Join(h.coursesDir, c.Dir))
	}
	return nil
}

func (h *Handler) loadAndRegisterCourse(slug string) (*domain.Course, error) {
	c, err := courseparser.LoadOne(h.coursesDir, slug)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	h.courses[c.Slug] = c
	h.mu.Unlock()
	return c, nil
}

func (h *Handler) loadAndRegisterCatalog(dirSlug string) (*domain.Catalog, error) {
	cat, err := courseparser.LoadCatalogManifest(h.coursesDir, dirSlug)
	if err != nil {
		return nil, err
	}
	// legacy layout: register any course folders physically nested under the catalog
	nested, err := courseparser.LoadNestedCourses(h.coursesDir, dirSlug)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	for _, c := range nested {
		h.courses[c.Slug] = c
	}
	h.catalogs[cat.Slug] = cat
	courseparser.ResolveCatalogCourses(cat, h.courses)
	h.mu.Unlock()
	return cat, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
