package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/paintingpromisesss/courseforge/internal/course"
)

func (h *Handler) uploadCourse(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		h.writeError(w, http.StatusBadRequest, "no files provided")
		return
	}

	// derive slug from first file path component
	firstPath := filepath.ToSlash(files[0].Filename)
	parts := strings.SplitN(firstPath, "/", 2)
	if len(parts) < 2 {
		h.writeError(w, http.StatusBadRequest, "expected files with relative paths (use webkitdirectory input)")
		return
	}
	slug := parts[0]

	destDir := filepath.Join(h.coursesDir, slug)
	if _, err := os.Stat(destDir); err == nil {
		h.writeError(w, http.StatusConflict, fmt.Sprintf("course %q already exists", slug))
		return
	}

	// write to temp dir first for atomicity
	tmpDir, err := os.MkdirTemp("", "cf-import-*")
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	for _, fh := range files {
		rel := filepath.ToSlash(fh.Filename)
		if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
			h.writeError(w, http.StatusBadRequest, "invalid file path: "+fh.Filename)
			return
		}
		target := filepath.Join(tmpDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to create directory")
			return
		}
		src, err := fh.Open()
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to read uploaded file")
			return
		}
		dst, err := os.Create(target)
		if err != nil {
			src.Close()
			h.writeError(w, http.StatusInternalServerError, "failed to write file")
			return
		}
		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to save file")
			return
		}
	}

	// validate: course.yaml must exist
	courseYAML := filepath.Join(tmpDir, slug, "course.yaml")
	if _, err := os.Stat(courseYAML); err != nil {
		h.writeError(w, http.StatusBadRequest, "course.yaml not found in uploaded folder")
		return
	}

	if err := os.Rename(filepath.Join(tmpDir, slug), destDir); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to install course")
		return
	}

	c, err := h.loadAndRegisterCourse(slug)
	if err != nil {
		// roll back the directory move
		_ = os.RemoveAll(destDir)
		h.writeError(w, http.StatusBadRequest, "invalid course: "+err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]string{"slug": c.Slug})
}

func (h *Handler) importCourse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		h.writeError(w, http.StatusBadRequest, "JSON body with 'path' required")
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "path not found")
		return
	}
	if !info.IsDir() {
		h.writeError(w, http.StatusBadRequest, "path must be a directory")
		return
	}

	// validate course.yaml exists
	if _, err := os.Stat(filepath.Join(req.Path, "course.yaml")); err != nil {
		h.writeError(w, http.StatusBadRequest, "course.yaml not found at path")
		return
	}

	slug := filepath.Base(req.Path)
	destDir := filepath.Join(h.coursesDir, slug)
	if _, err := os.Stat(destDir); err == nil {
		h.writeError(w, http.StatusConflict, fmt.Sprintf("course %q already exists", slug))
		return
	}

	if err := copyDir(req.Path, destDir); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to copy course: "+err.Error())
		return
	}

	c, err := h.loadAndRegisterCourse(slug)
	if err != nil {
		_ = os.RemoveAll(destDir)
		h.writeError(w, http.StatusBadRequest, "invalid course: "+err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]string{"slug": c.Slug})
}

func (h *Handler) loadAndRegisterCourse(slug string) (*course.Course, error) {
	c, err := course.LoadOne(h.coursesDir, slug)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	h.courses[c.Slug] = c
	h.mu.Unlock()
	return c, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
