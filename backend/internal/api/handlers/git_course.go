package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

// gitCourseTarget resolves {courseSlug} (course or catalog) to its git-imported
// top-level directory. ok=false means "not a git-managed course" → 404.
func (h *Handler) gitCourseTarget(r *http.Request) (dirSlug, dir string, src repo.CourseSource, ok bool) {
	slug := chi.URLParam(r, "courseSlug")
	h.mu.RLock()
	var rel string
	if c := h.courses[slug]; c != nil {
		rel = c.Dir
	} else if cat := h.catalogs[slug]; cat != nil {
		rel = cat.Dir
	}
	h.mu.RUnlock()
	if rel == "" {
		return "", "", src, false
	}
	dirSlug = strings.SplitN(rel, "/", 2)[0] // source is tracked per top-level dir
	sources, err := h.sources.All(r.Context())
	if err != nil {
		return "", "", src, false
	}
	src, found := sources[dirSlug]
	if !found {
		return "", "", src, false
	}
	return dirSlug, filepath.Join(h.coursesDir, filepath.FromSlash(dirSlug)), src, true
}

func (h *Handler) gitUnavailable(w http.ResponseWriter) bool {
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return true
	}
	return false
}

// writeGitError maps the wrapper's dirty-tree refusal to 409, other git
// failures to 400.
func (h *Handler) writeGitError(w http.ResponseWriter, err error) {
	var ge *git.Error
	if errors.As(err, &ge) && strings.Contains(ge.Stderr, "uncommitted changes") {
		h.writeError(w, http.StatusConflict, ge.Stderr)
		return
	}
	h.writeError(w, http.StatusBadRequest, err.Error())
}

// @Summary List remote branches of a git-imported course
// @Tags git
// @Produce json
// @Param courseSlug path string true "Course or catalog slug"
// @Success 200 {object} dto.GitBranchesResp
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /courses/{courseSlug}/git/branches [get]
func (h *Handler) getGitBranches(w http.ResponseWriter, r *http.Request) {
	if h.gitUnavailable(w) {
		return
	}
	_, dir, src, ok := h.gitCourseTarget(r)
	if !ok {
		h.writeError(w, http.StatusNotFound, "course is not managed by git")
		return
	}
	if err := h.gitSvc.Fetch(r.Context(), dir); err != nil {
		h.writeError(w, http.StatusBadGateway, "failed to fetch from origin: "+err.Error())
		return
	}
	branches, err := h.gitSvc.RemoteBranches(r.Context(), dir)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to list branches: "+err.Error())
		return
	}
	current, _ := h.gitSvc.CurrentBranch(r.Context(), dir)
	h.writeJSON(w, http.StatusOK, dto.GitBranchesResp{
		Branches: branches,
		Current:  current,
		Source: &dto.CourseSourceDTO{
			Repo: src.Repo, Branch: src.Branch, Commit: src.Commit, ImportedAt: src.ImportedAt,
		},
	})
}

// @Summary Git state of an imported course directory
// @Tags git
// @Produce json
// @Param courseSlug path string true "Course or catalog slug"
// @Success 200 {object} dto.GitStatusResp
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /courses/{courseSlug}/git/status [get]
func (h *Handler) getGitStatus(w http.ResponseWriter, r *http.Request) {
	if h.gitUnavailable(w) {
		return
	}
	_, dir, _, ok := h.gitCourseTarget(r)
	if !ok {
		h.writeError(w, http.StatusNotFound, "course is not managed by git")
		return
	}
	branch, _ := h.gitSvc.CurrentBranch(r.Context(), dir)
	commit, _ := h.gitSvc.HeadCommit(r.Context(), dir)
	dirty, _ := h.gitSvc.Dirty(r.Context(), dir)
	h.writeJSON(w, http.StatusOK, dto.GitStatusResp{Branch: branch, Commit: commit, Dirty: dirty})
}

// @Summary Switch a git-imported course to another branch
// @Description Refuses (409) when the working tree is dirty unless force is set.
// @Description If the checked-out content fails to parse, the previous branch is
// @Description restored and 422 is returned.
// @Tags git
// @Accept json
// @Produce json
// @Param courseSlug path string true "Course or catalog slug"
// @Param body body dto.GitCheckoutReq true "Target branch and force flag"
// @Success 200 {object} dto.GitImportResp
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /courses/{courseSlug}/git/checkout [post]
func (h *Handler) postGitCheckout(w http.ResponseWriter, r *http.Request) {
	if h.gitUnavailable(w) {
		return
	}
	dirSlug, dir, src, ok := h.gitCourseTarget(r)
	if !ok {
		h.writeError(w, http.StatusNotFound, "course is not managed by git")
		return
	}
	var req dto.GitCheckoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Branch) == "" {
		h.writeError(w, http.StatusBadRequest, "branch is required")
		return
	}
	branch := strings.TrimSpace(req.Branch)

	prevBranch, err := h.gitSvc.CurrentBranch(r.Context(), dir)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read current branch: "+err.Error())
		return
	}
	if err := h.gitSvc.CheckoutBranch(r.Context(), dir, branch, req.Force); err != nil {
		h.writeGitError(w, err)
		return
	}
	if err := h.revalidateGitContent(dirSlug); err != nil {
		// roll back to the previous branch and re-register it (best effort)
		_ = h.gitSvc.CheckoutBranch(r.Context(), dir, prevBranch, true)
		_ = h.revalidateGitContent(dirSlug)
		h.writeError(w, http.StatusUnprocessableEntity, "branch content is invalid: "+err.Error())
		return
	}
	h.finishGitOp(r, w, dirSlug, dir, src, branch)
}

// @Summary Pull the latest commits of the current branch (fast-forward only)
// @Tags git
// @Accept json
// @Produce json
// @Param courseSlug path string true "Course or catalog slug"
// @Param body body dto.GitCheckoutReq false "force: discard local edits"
// @Success 200 {object} dto.GitImportResp
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /courses/{courseSlug}/git/pull [post]
func (h *Handler) postGitPull(w http.ResponseWriter, r *http.Request) {
	if h.gitUnavailable(w) {
		return
	}
	dirSlug, dir, src, ok := h.gitCourseTarget(r)
	if !ok {
		h.writeError(w, http.StatusNotFound, "course is not managed by git")
		return
	}
	var req dto.GitCheckoutReq
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	prevCommit, err := h.gitSvc.HeadCommit(r.Context(), dir)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read HEAD: "+err.Error())
		return
	}
	if !req.Force {
		dirty, err := h.gitSvc.Dirty(r.Context(), dir)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to read status: "+err.Error())
			return
		}
		if dirty {
			h.writeError(w, http.StatusConflict, "working tree has uncommitted changes; commit or discard them first")
			return
		}
	} else {
		if err := h.gitSvc.ResetHard(r.Context(), dir, "HEAD"); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to discard local changes: "+err.Error())
			return
		}
	}
	if err := h.gitSvc.PullFF(r.Context(), dir); err != nil {
		h.writeGitError(w, err)
		return
	}
	if err := h.revalidateGitContent(dirSlug); err != nil {
		_ = h.gitSvc.ResetHard(r.Context(), dir, prevCommit)
		_ = h.revalidateGitContent(dirSlug)
		h.writeError(w, http.StatusUnprocessableEntity, "pulled content is invalid: "+err.Error())
		return
	}
	branch, _ := h.gitSvc.CurrentBranch(r.Context(), dir)
	h.finishGitOp(r, w, dirSlug, dir, src, branch)
}

// revalidateGitContent re-parses the course/catalog at dirSlug after its files
// changed on disk, replacing the registered objects.
func (h *Handler) revalidateGitContent(dirSlug string) error {
	if isCatalogDir(h.coursesDir, dirSlug) {
		_, err := h.loadAndRegisterCatalog(dirSlug)
		return err
	}
	_, err := h.loadAndRegisterCourse(dirSlug)
	return err
}

func isCatalogDir(coursesDir, dirSlug string) bool {
	return fileExists(filepath.Join(coursesDir, dirSlug, "catalog.yaml"))
}

// finishGitOp records the new branch/commit in the source store and responds.
func (h *Handler) finishGitOp(r *http.Request, w http.ResponseWriter, dirSlug, dir string, src repo.CourseSource, branch string) {
	commit, _ := h.gitSvc.HeadCommit(r.Context(), dir)
	src.Branch = branch
	src.Commit = commit
	_ = h.sources.Set(r.Context(), dirSlug, src)
	h.writeJSON(w, http.StatusOK, dto.GitImportResp{Slug: dirSlug, Branch: branch, Commit: commit})
}
