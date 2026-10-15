package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func (h *Handler) gitAuthStatus(ctx context.Context) dto.GitAuthStatus {
	status := dto.GitAuthStatus{}
	// git-level credential helpers (OS keychain, store) — shown so users
	// understand how private repos clone without a token in Settings
	if helpers, err := h.gitSvc.CredentialHelper(ctx, ""); err == nil && len(helpers) > 0 {
		status.CredentialHelper = helpers
	}
	a, err := h.gitAuth.Load(ctx)
	if err != nil || a == nil || a.Token == "" {
		return status
	}
	status.Configured = true
	status.Username = a.Username
	status.TokenMasked = repo.MaskToken(a.Token)
	return status
}

// @Summary Get GitHub auth status
// @Description Reports whether a token is configured; the raw token is never returned.
// @Tags git
// @Produce json
// @Success 200 {object} dto.GitAuthStatus
// @Router /git/auth [get]
func (h *Handler) getGitAuth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, h.gitAuthStatus(r.Context()))
}

// @Summary Set or clear the GitHub token
// @Description An empty token clears the stored credentials.
// @Tags git
// @Accept json
// @Produce json
// @Param body body dto.PatchGitAuthReq true "Token and optional username"
// @Success 200 {object} dto.GitAuthStatus
// @Failure 400 {object} map[string]string
// @Router /git/auth [patch]
func (h *Handler) patchGitAuth(w http.ResponseWriter, r *http.Request) {
	var req dto.PatchGitAuthReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		if err := h.gitAuth.Delete(); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to clear token")
			return
		}
		h.gitSvc.SetToken("")
		h.writeJSON(w, http.StatusOK, h.gitAuthStatus(r.Context()))
		return
	}
	auth := &repo.GitAuth{Token: req.Token, Username: req.Username}
	if err := h.gitAuth.Save(r.Context(), auth); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save token")
		return
	}
	h.gitSvc.SetToken(req.Token)
	h.writeJSON(w, http.StatusOK, h.gitAuthStatus(r.Context()))
}

// @Summary Validate the stored token against the GitHub API
// @Tags git
// @Produce json
// @Success 200 {object} dto.GitAuthTestResp
// @Router /git/auth/test [post]
func (h *Handler) testGitAuth(w http.ResponseWriter, r *http.Request) {
	a, err := h.gitAuth.Load(r.Context())
	if err != nil || a == nil || a.Token == "" {
		h.writeJSON(w, http.StatusOK, dto.GitAuthTestResp{OK: false, Error: "no token configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		h.writeJSON(w, http.StatusOK, dto.GitAuthTestResp{OK: false, Error: err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.writeJSON(w, http.StatusOK, dto.GitAuthTestResp{OK: false, Error: "network error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		var user struct {
			Login string `json:"login"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&user)
		h.writeJSON(w, http.StatusOK, dto.GitAuthTestResp{OK: true, Login: user.Login})
	case http.StatusUnauthorized, http.StatusForbidden:
		h.writeJSON(w, http.StatusOK, dto.GitAuthTestResp{OK: false, Error: "invalid token"})
	default:
		h.writeJSON(w, http.StatusOK, dto.GitAuthTestResp{OK: false, Error: "github api returned " + resp.Status})
	}
}
