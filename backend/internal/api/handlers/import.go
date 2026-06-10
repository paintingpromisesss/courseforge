package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	courseparser "github.com/paintingpromisesss/courseforge/internal/infrastructure/parser/course"
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

	relPaths, err := uploadedRelativePaths(files, r.MultipartForm.Value["paths"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// derive the uploaded root folder from the first relative path component
	firstPath := relPaths[0]
	parts := strings.SplitN(firstPath, "/", 2)
	if len(parts) < 2 {
		h.writeError(w, http.StatusBadRequest, "expected files with relative paths (use webkitdirectory input)")
		return
	}
	rootDir := parts[0]

	// write to temp dir first for atomicity
	tmpDir, err := os.MkdirTemp("", "cf-import-*")
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	for i, fh := range files {
		rel := relPaths[i]
		if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
			h.writeError(w, http.StatusBadRequest, "invalid file path: "+rel)
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
	sourceDir := filepath.Join(tmpDir, rootDir)
	courseYAML := filepath.Join(sourceDir, "course.yaml")
	if _, err := os.Stat(courseYAML); err != nil {
		h.writeError(w, http.StatusBadRequest, "course.yaml not found in uploaded folder")
		return
	}

	slug, status, msg := h.inspectImportedCourse(sourceDir)
	if status != 0 {
		h.writeError(w, status, msg)
		return
	}

	destDir := filepath.Join(h.coursesDir, slug)
	if err := os.Rename(sourceDir, destDir); err != nil {
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

	h.writeJSON(w, http.StatusCreated, dto.SlugResp{Slug: c.Slug})
}

func uploadedRelativePaths(files []*multipart.FileHeader, paths []string) ([]string, error) {
	if len(paths) > 0 && len(paths) != len(files) {
		return nil, fmt.Errorf("files and paths count mismatch")
	}

	relPaths := make([]string, 0, len(files))
	root := ""

	for i, fh := range files {
		rel := filepath.ToSlash(fh.Filename)
		if len(paths) == len(files) {
			rel = filepath.ToSlash(paths[i])
		}
		rel = strings.TrimPrefix(rel, "./")

		parts := strings.SplitN(rel, "/", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("expected files with relative paths (use webkitdirectory input)")
		}
		if root == "" {
			root = parts[0]
		} else if parts[0] != root {
			return nil, fmt.Errorf("all uploaded files must come from the same root folder")
		}

		relPaths = append(relPaths, rel)
	}

	return relPaths, nil
}

func (h *Handler) importCourse(w http.ResponseWriter, r *http.Request) {
	var req dto.ImportCourseReq
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

	slug, status, msg := h.inspectImportedCourse(req.Path)
	if status != 0 {
		h.writeError(w, status, msg)
		return
	}

	destDir := filepath.Join(h.coursesDir, slug)
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

	h.writeJSON(w, http.StatusCreated, dto.SlugResp{Slug: c.Slug})
}

func (h *Handler) inspectImportedCourse(dir string) (string, int, string) {
	c, err := courseparser.LoadOne(filepath.Dir(dir), filepath.Base(dir))
	if err != nil {
		return "", http.StatusBadRequest, "invalid course: " + err.Error()
	}

	if h.getCourseBySlug(c.Slug) != nil {
		return "", http.StatusConflict, fmt.Sprintf("course slug %q already exists", c.Slug)
	}

	destDir := filepath.Join(h.coursesDir, c.Slug)
	if _, err := os.Stat(destDir); err == nil {
		return "", http.StatusConflict, fmt.Sprintf("course slug %q already exists", c.Slug)
	} else if !os.IsNotExist(err) {
		return "", http.StatusInternalServerError, "failed to check course destination"
	}

	return c.Slug, 0, ""
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
