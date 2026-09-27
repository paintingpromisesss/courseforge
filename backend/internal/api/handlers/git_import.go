package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

// validateRepoURL accepts https://github.com/<owner>/<repo>(.git) and, for
// local testing, existing filesystem paths (git handles them as file remotes).
// Returns the URL unchanged when accepted.
func validateRepoURL(raw string) (string, bool) {
	if raw == "" || strings.HasPrefix(raw, "-") {
		return "", false
	}
	// Local directories first: on Windows "C:\..." parses as a URL with a
	// drive-letter scheme, so stat before treating raw as a URL.
	if fi, err := os.Stat(raw); err == nil && fi.IsDir() {
		return raw, true
	}
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		if !strings.EqualFold(u.Scheme, "https") && !strings.EqualFold(u.Scheme, "http") {
			return "", false
		}
		if !strings.EqualFold(u.Hostname(), "github.com") {
			return "", false
		}
		p := strings.Trim(u.Path, "/")
		p = strings.TrimSuffix(p, ".git")
		if segs := strings.Split(p, "/"); len(segs) != 2 || segs[0] == "" || segs[1] == "" {
			return "", false
		}
		return raw, true
	}
	return "", false
}

// classifyCloneError maps git stderr to the HTTP status we want to answer with.
// Anything that smells like "repo not reachable without credentials" is a 401
// so the UI can offer the token field; everything else is a plain 400.
func classifyCloneError(err error) int {
	var ge *git.Error
	if !errors.As(err, &ge) {
		return http.StatusBadRequest
	}
	low := strings.ToLower(ge.Stderr)
	for _, marker := range []string{
		"authentication failed",
		"could not read username",
		"terminal prompts disabled",
		"repository not found",
		"access denied",
		"http basic",
	} {
		if strings.Contains(low, marker) {
			return http.StatusUnauthorized
		}
	}
	return http.StatusBadRequest
}

// manifestSlug reads the slug out of course.yaml or catalog.yaml at dir root.
func manifestSlug(dir string) string {
	for _, name := range []string{"catalog.yaml", "course.yaml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var m struct {
			Slug string `yaml:"slug"`
		}
		if yaml.Unmarshal(data, &m) == nil && m.Slug != "" {
			return m.Slug
		}
	}
	return ""
}

// @Summary Import a course or catalog from a GitHub repository
// @Description Clones the repository (manifest must be at the repo root) and installs
// @Description it into the courses directory, preserving .git for branch switching.
// @Tags git
// @Accept json
// @Produce json
// @Param body body dto.GitImportReq true "Repository URL and optional branch"
// @Success 201 {object} dto.GitImportResp
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /git/import [post]
func (h *Handler) gitImport(w http.ResponseWriter, r *http.Request) {
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return
	}
	var req dto.GitImportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	srcURL, ok := validateRepoURL(strings.TrimSpace(req.URL))
	if !ok {
		h.writeError(w, http.StatusBadRequest, "expected a https://github.com/owner/repo URL")
		return
	}

	tmpDir, err := os.MkdirTemp("", "cf-git-import-*")
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create temp dir")
		return
	}
	defer os.RemoveAll(tmpDir)

	cloneDir := filepath.Join(tmpDir, "repo")
	if err := h.gitSvc.Clone(r.Context(), srcURL, cloneDir, strings.TrimSpace(req.Branch)); err != nil {
		if status := classifyCloneError(err); status == http.StatusUnauthorized {
			h.writeError(w, status, "repository is private or unavailable — add a GitHub token in Settings")
		} else {
			h.writeError(w, status, "clone failed: "+err.Error())
		}
		return
	}

	if !fileExists(filepath.Join(cloneDir, "course.yaml")) && !fileExists(filepath.Join(cloneDir, "catalog.yaml")) {
		h.writeError(w, http.StatusBadRequest, "neither course.yaml nor catalog.yaml found in repository root")
		return
	}
	// slug must equal folder name: rename the clone dir to the declared slug
	// before importFromDir moves it into coursesDir.
	slug := manifestSlug(cloneDir)
	if slug == "" {
		h.writeError(w, http.StatusBadRequest, "manifest has no slug")
		return
	}
	// moveDir, not os.Rename: on Windows a freshly cloned root can be held by
	// an external scanner/watcher (rename → "Access is denied"); moveDir falls
	// back to a recursive copy that works regardless.
	namedDir := filepath.Join(tmpDir, slug)
	if err := moveDir(cloneDir, namedDir); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to prepare import")
		return
	}

	imported, herr := h.importFromDir(namedDir)
	if herr != nil {
		h.writeError(w, herr.status, herr.msg)
		return
	}

	destDir := filepath.Join(h.coursesDir, imported)
	branch, _ := h.gitSvc.CurrentBranch(r.Context(), destDir)
	commit, _ := h.gitSvc.HeadCommit(r.Context(), destDir)
	if err := h.sources.Set(r.Context(), imported, repo.CourseSource{
		Repo:       srcURL,
		Branch:     branch,
		Commit:     commit,
		ImportedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to record course source")
		return
	}
	h.writeJSON(w, http.StatusCreated, dto.GitImportResp{Slug: imported, Branch: branch, Commit: commit})
}
