package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// @Summary Execute code
// @Tags run
// @Accept json
// @Produce json
// @Param request body dto.RunReq true "Run request"
// @Success 200 {object} dto.RunResp
// @Failure 400 {object} map[string]string
// @Router /run [post]
func (h *Handler) postRun(w http.ResponseWriter, r *http.Request) {
	var req dto.RunReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Language == "" || req.Code == "" {
		h.writeError(w, http.StatusBadRequest, "language and code are required")
		return
	}

	result, err := h.runner.Run(runner.RunRequest{
		Language: req.Language,
		Code:     req.Code,
		TestCode: req.TestCode,
		Timeout:  time.Duration(req.TimeoutSec) * time.Second,
	})
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, dto.RunResp{
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		ExitCode:   result.ExitCode,
		DurationMs: result.Duration.Milliseconds(),
		TimedOut:   result.TimedOut,
	})
}

// @Summary List registered language drivers
// @Tags runners
// @Produce json
// @Success 200 {object} map[string]dto.RunnerDriver
// @Router /runners [get]
func (h *Handler) listRunners(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, dto.ToRunnerDrivers(h.runner.Drivers()))
}

// @Summary Detect and test a language runner on the host
// @Tags runners
// @Produce json
// @Param lang path string true "Language key (e.g. go)"
// @Success 200 {object} dto.DetectResp
// @Failure 404 {object} map[string]string
// @Router /runners/{lang}/detect [post]
func (h *Handler) detectRunner(w http.ResponseWriter, r *http.Request) {
	lang := chi.URLParam(r, "lang")
	if !h.runner.HasDriver(lang) {
		h.writeError(w, http.StatusNotFound, "driver not found")
		return
	}
	res := h.runner.Detect(lang)
	h.writeJSON(w, http.StatusOK, dto.DetectResp{
		Status:  string(res.Status),
		Binary:  res.Binary,
		Version: res.Version,
		Message: res.Message,
	})
}

// @Summary Partially update an existing language driver
// @Tags runners
// @Accept json
// @Param lang path string true "Language key (e.g. go)"
// @Param request body dto.PatchRunnerReq true "Fields to update"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /runners/{lang} [patch]
func (h *Handler) patchRunner(w http.ResponseWriter, r *http.Request) {
	lang := chi.URLParam(r, "lang")
	drivers := h.runner.Drivers()
	d, ok := drivers[lang]
	if !ok {
		h.writeError(w, http.StatusNotFound, "driver not found")
		return
	}

	var req dto.PatchRunnerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RunCmd != nil {
		if len(*req.RunCmd) == 0 {
			h.writeError(w, http.StatusBadRequest, "run_cmd cannot be empty")
			return
		}
		if _, err := exec.LookPath((*req.RunCmd)[0]); err != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("binary %q not found in PATH", (*req.RunCmd)[0]))
			return
		}
		d.RunCmd = *req.RunCmd
	}
	if req.TestCmd != nil {
		d.TestCmd = *req.TestCmd
	}

	h.runner.AddDriver(lang, d)
	w.WriteHeader(http.StatusNoContent)
}
