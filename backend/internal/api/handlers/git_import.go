package handlers

import (
	"context"
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

// importOneRepo clones url and installs it, returning the per-URL outcome
// for the batch endpoint. gitImport answers with HTTP statuses; this returns
// the same classification as data.
func (h *Handler) importOneRepo(ctx context.Context, rawURL string) dto.GitImportBatchItem {
	item := dto.GitImportBatchItem{URL: rawURL}
	srcURL, ok := validateRepoURL(strings.TrimSpace(rawURL))
	if !ok {
		item.Error = "expected a https://github.com/owner/repo URL"
		return item
	}
	item.URL = srcURL

	tmpDir, err := os.MkdirTemp("", "cf-git-import-*")
	if err != nil {
		item.Error = "failed to create temp dir"
		return item
	}
	defer os.RemoveAll(tmpDir)

	cloneDir := filepath.Join(tmpDir, "repo")
	if err := h.gitSvc.Clone(ctx, srcURL, cloneDir, ""); err != nil {
		if classifyCloneError(err) == http.StatusUnauthorized {
			item.Error = "repository is private or unavailable — add a GitHub token in Settings"
		} else {
			item.Error = "clone failed: " + err.Error()
		}
		return item
	}

	if !fileExists(filepath.Join(cloneDir, "course.yaml")) && !fileExists(filepath.Join(cloneDir, "catalog.yaml")) {
		item.Error = "neither course.yaml nor catalog.yaml found in repository root"
		return item
	}
	slug := manifestSlug(cloneDir)
	if slug == "" {
		item.Error = "manifest has no slug"
		return item
	}
	namedDir := filepath.Join(tmpDir, slug)
	if err := moveDir(cloneDir, namedDir); err != nil {
		item.Error = "failed to prepare import"
		return item
	}
	imported, herr := h.importFromDir(namedDir)
	if herr != nil {
		item.Error = herr.msg
		return item
	}
	destDir := filepath.Join(h.coursesDir, imported)
	branch, _ := h.gitSvc.CurrentBranch(ctx, destDir)
	commit, _ := h.gitSvc.HeadCommit(ctx, destDir)
	if err := h.sources.Set(ctx, imported, repo.CourseSource{
		Repo:       srcURL,
		Branch:     branch,
		Commit:     commit,
		ImportedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		item.Error = "failed to record course source"
		return item
	}
	item.OK = true
	item.Slug = imported
	item.Branch = branch
	item.Commit = commit
	return item
}

// @Summary Import several courses from GitHub in one batch
// @Description Clones every URL (default branch), validates the manifest at the repo
// @Description root and installs each valid repository. One invalid URL never stops
// @Description the rest: every outcome is reported per-URL. Branch is not settable
// @Description here — switch branches in the course UI after import.
// @Tags git
// @Accept json
// @Produce json
// @Param body body dto.GitImportBatchReq true "Repository URLs"
// @Success 200 {object} dto.GitImportBatchResp
// @Failure 400 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /git/import/batch [post]
func (h *Handler) gitImportBatch(w http.ResponseWriter, r *http.Request) {
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return
	}
	var req dto.GitImportBatchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.URLs) == 0 {
		h.writeError(w, http.StatusBadRequest, "no urls provided")
		return
	}
	if len(req.URLs) > 50 {
		h.writeError(w, http.StatusBadRequest, "too many urls (max 50)")
		return
	}

	// dedupe by trimmed URL — the same repo twice would collide on the slug
	seen := make(map[string]bool, len(req.URLs))
	urls := make([]string, 0, len(req.URLs))
	results := make([]dto.GitImportBatchItem, 0, len(req.URLs))
	for _, raw := range req.URLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if seen[raw] {
			results = append(results, dto.GitImportBatchItem{
				URL:   raw,
				Error: "duplicate URL in this batch",
			})
			continue
		}
		seen[raw] = true
		urls = append(urls, raw)
	}

	for _, raw := range urls {
		results = append(results, h.importOneRepo(r.Context(), raw))
	}
	h.writeJSON(w, http.StatusOK, dto.GitImportBatchResp{Results: results})
}
