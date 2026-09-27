package handlers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	csync "github.com/paintingpromisesss/courseforge/internal/infrastructure/sync"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func (h *Handler) engineOrWriteErr(w http.ResponseWriter) bool {
	h.mu.RLock()
	e := h.syncEngine
	h.mu.RUnlock()
	if e == nil {
		h.writeError(w, http.StatusServiceUnavailable, "sync engine unavailable")
		return false
	}
	return true
}

// engine returns the sync engine; callers checked availability via
// engineOrWriteErr first.
func (h *Handler) engine() *csync.Engine {
	return h.syncEngine
}

func syncConfigResp(cfg *repo.SyncConfig) dto.SyncConfigResp {
	if cfg == nil {
	return dto.SyncConfigResp{Branch: "main"}
	}
	r := dto.SyncConfigResp{
		RemoteURL: cfg.RemoteURL,
		Branch:    cfg.Branch,
		Enabled:   cfg.Enabled,
		Exclude:   cfg.Exclude,
	}
	if r.Branch == "" {
		r.Branch = "main"
	}
	r.Triggers = dto.SyncTriggers{
		OnProgress:    cfg.Triggers.OnProgress,
		IntervalMin:   cfg.Triggers.IntervalMin,
		OnStartupPull: cfg.Triggers.OnStartupPull,
	}
	if cfg.LastSync != "" {
		r.LastSync = cfg.LastSync
	}
	return r
}

// @Summary Get cloud sync configuration
// @Tags sync
// @Produce json
// @Success 200 {object} dto.SyncConfigResp
// @Router /sync/config [get]
func (h *Handler) getSyncConfig(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	cfg, err := h.syncEngine.LoadConfig(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load sync config")
		return
	}
	h.writeJSON(w, http.StatusOK, syncConfigResp(cfg))
}

// @Summary Update cloud sync configuration
// @Description Nil fields stay unchanged. Changing remote_url or branch resets the local mirror so the next push re-initializes it.
// @Tags sync
// @Accept json
// @Produce json
// @Param body body dto.PatchSyncConfigReq true "Fields to update"
// @Success 200 {object} dto.SyncConfigResp
// @Failure 400 {object} map[string]string
// @Router /sync/config [patch]
func (h *Handler) patchSyncConfig(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	var req dto.PatchSyncConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RemoteURL != nil && *req.RemoteURL != "" && strings.HasPrefix(*req.RemoteURL, "-") {
		h.writeError(w, http.StatusBadRequest, "invalid remote url")
		return
	}
	if req.Branch != nil && (*req.Branch == "" || strings.HasPrefix(*req.Branch, "-")) {
		h.writeError(w, http.StatusBadRequest, "invalid branch")
		return
	}
	cfg, err := h.syncEngine.LoadConfig(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load sync config")
		return
	}
	if cfg == nil {
		cfg = &repo.SyncConfig{Branch: "main"}
	}
	// remote/branch change → wipe mirror so the next push re-inits it
	if (req.RemoteURL != nil && *req.RemoteURL != cfg.RemoteURL) ||
		(req.Branch != nil && *req.Branch != cfg.Branch) {
		_ = h.syncEngine.ResetMirror()
	}
	if req.RemoteURL != nil {
		cfg.RemoteURL = *req.RemoteURL
	}
	if req.Branch != nil {
		cfg.Branch = *req.Branch
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.Exclude != nil {
		cfg.Exclude = *req.Exclude
	}
	if req.Triggers != nil {
		cfg.Triggers = repo.SyncTriggers{
			OnProgress:    req.Triggers.OnProgress,
			IntervalMin:   req.Triggers.IntervalMin,
			OnStartupPull: req.Triggers.OnStartupPull,
		}
	}
	if err := h.syncEngine.SaveConfig(r.Context(), cfg); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save sync config")
		return
	}
	h.syncEngine.SetTriggers(cfg)
	h.writeJSON(w, http.StatusOK, syncConfigResp(cfg))
}

// @Summary Push a snapshot to the cloud vault
// @Description Snapshots all local courses and progress into the mirror repo and pushes it. Retries up to 3 times when the remote moved; never force-pushes.
// @Tags sync
// @Produce json
// @Success 200 {object} dto.SyncStatusResp
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /sync/push [post]
func (h *Handler) postSyncPush(w http.ResponseWriter, r *http.Request) {
	h.doSyncOp(w, r, func(ctx context.Context) error { return h.engine().Push(ctx) })
}

// @Summary Pull the latest cloud snapshot
// @Description Copies the cloud snapshot back. Never deletes local courses.
// @Tags sync
// @Produce json
// @Success 200 {object} dto.SyncStatusResp
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /sync/pull [post]
func (h *Handler) postSyncPull(w http.ResponseWriter, r *http.Request) {
	h.doSyncOp(w, r, func(ctx context.Context) error { return h.engine().Pull(ctx) })
}

func (h *Handler) doSyncOp(w http.ResponseWriter, r *http.Request, op func(context.Context) error) {
	if !h.engineOrWriteErr(w) {
		return
	}
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return
	}
	if configured, _ := h.syncEngine.Configured(r.Context()); !configured {
		h.writeError(w, http.StatusBadRequest, "sync not configured")
		return
	}
	if h.syncEngine.Busy() {
		h.writeError(w, http.StatusConflict, "sync is already running")
		return
	}
	if err := op(r.Context()); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeSyncStatus(w, r)
}

// @Summary Get sync status
// @Tags sync
// @Produce json
// @Success 200 {object} dto.SyncStatusResp
// @Router /sync/status [get]
func (h *Handler) getSyncStatus(w http.ResponseWriter, r *http.Request) {
	h.writeSyncStatus(w, r)
}

func (h *Handler) writeSyncStatus(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	configured, err := h.syncEngine.Configured(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load sync config")
		return
	}
	st, err := h.syncEngine.Status(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load sync status")
		return
	}
	enabled := false
	if cfg, _ := h.syncEngine.LoadConfig(r.Context()); cfg != nil {
		enabled = cfg.Enabled
	}
	resp := dto.SyncStatusResp{
		Configured:     configured,
		Enabled:        enabled,
		Syncing:        st.Syncing,
		LastSync:       st.LastSync,
		Branch:         st.Branch,
		Commit:         st.HeadCommit,
		PendingImports: st.PendingImports,
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// @Summary List sync history
// @Tags sync
// @Produce json
// @Param limit query int false "Max commits (default 50)"
// @Success 200 {array} dto.SyncHistoryItem
// @Router /sync/history [get]
func (h *Handler) getSyncHistory(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	commits, err := h.syncEngine.History(r.Context(), limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read sync history")
		return
	}
	items := make([]dto.SyncHistoryItem, 0, len(commits))
	for _, c := range commits {
		items = append(items, dto.SyncHistoryItem{
			Commit:  c.Hash,
			Author:  c.Author,
			Subject: c.Subject,
			Time:    c.Time.UTC().Format(time.RFC3339),
		})
	}
	if items == nil {
		items = []dto.SyncHistoryItem{}
	}
	h.writeJSON(w, http.StatusOK, items)
}

// @Summary Files changed in a sync commit
// @Tags sync
// @Produce json
// @Param commit path string true "Commit hash"
// @Success 200 {object} dto.SyncCommitFilesResp
// @Failure 400 {object} map[string]string
// @Router /sync/history/{commit} [get]
func (h *Handler) getSyncCommitFiles(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	commit := chi.URLParam(r, "commit")
	if !isHexCommit(commit) {
		h.writeError(w, http.StatusBadRequest, "invalid commit hash")
		return
	}
	files, err := h.syncEngine.CommitFiles(r.Context(), commit)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "commit not found")
		return
	}
	if files == nil {
		files = []string{}
	}
	h.writeJSON(w, http.StatusOK, dto.SyncCommitFilesResp{Files: files})
}

// @Summary Roll back local state to a sync commit
// @Tags sync
// @Accept json
// @Produce json
// @Param body body dto.RollbackReq true "Commit to roll back to"
// @Success 200 {object} dto.SyncStatusResp
// @Failure 400 {object} map[string]string
// @Router /sync/rollback [post]
func (h *Handler) postSyncRollback(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return
	}
	var req dto.RollbackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !isHexCommit(req.Commit) {
		h.writeError(w, http.StatusBadRequest, "invalid commit hash")
		return
	}
	if configured, _ := h.syncEngine.Configured(r.Context()); !configured {
		h.writeError(w, http.StatusBadRequest, "sync not configured")
		return
	}
	if h.syncEngine.Busy() {
		h.writeError(w, http.StatusConflict, "sync is already running")
		return
	}
	if err := h.syncEngine.Rollback(r.Context(), req.Commit); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeSyncStatus(w, r)
}

// @Summary Re-clone courses imported from their own git remotes
// @Description For every pending import (recorded in cloud sources but missing locally) clones the source repo into courses dir and re-registers it.
// @Tags sync
// @Produce json
// @Success 200 {object} object {restored=[]string}
// @Router /sync/restore-imports [post]
func (h *Handler) postSyncRestoreImports(w http.ResponseWriter, r *http.Request) {
	if !h.engineOrWriteErr(w) {
		return
	}
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return
	}
	st, err := h.syncEngine.Status(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load sync status")
		return
	}
	sources, err := h.sources.All(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load course sources")
		return
	}
	var restored []string
	for _, dir := range st.PendingImports {
		src, ok := sources[dir]
		if !ok || src.Repo == "" {
			continue
		}
		dest := filepath.Join(h.coursesDir, filepath.FromSlash(dir))
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		if err := h.gitSvc.Clone(r.Context(), src.Repo, dest, src.Branch); err != nil {
			continue
		}
		if _, err := h.loadAndRegisterCourse(dir); err != nil {
			_ = os.RemoveAll(dest)
			continue
		}
		restored = append(restored, dir)
	}
	if restored == nil {
		restored = []string{}
	}
	h.writeJSON(w, http.StatusOK, map[string][]string{"restored": restored})
}

func isHexCommit(s string) bool {
	if len(s) < 4 || len(s) > 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
