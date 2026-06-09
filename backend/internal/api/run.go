package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/runner"
)

type RunReq struct {
	Language   string `json:"language" example:"go"`
	Code       string `json:"code"     example:"package main\nfunc main() {}"`
	TestCode   string `json:"test_code"`
	TimeoutSec int    `json:"timeout_sec" example:"10"`
}

type RunResp struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
}

// @Summary Execute code
// @Tags run
// @Accept json
// @Produce json
// @Param request body RunReq true "Run request"
// @Success 200 {object} RunResp
// @Failure 400 {object} map[string]string
// @Router /run [post]
func (h *Handler) postRun(w http.ResponseWriter, r *http.Request) {
	var req RunReq
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

	h.writeJSON(w, http.StatusOK, RunResp{
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
// @Success 200 {object} map[string]runner.LangDriver
// @Router /runners [get]
func (h *Handler) listRunners(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, h.runner.Drivers())
}

// AddDriverReq is the request body for adding a new language driver.
type AddDriverReq struct {
	Lang   string            `json:"lang"   example:"rust"`
	Driver runner.LangDriver `json:"driver"`
}

// PatchDriverReq is the request body for partially updating a language driver.
// Only non-nil fields are applied.
type PatchDriverReq struct {
	RunCmd  *[]string `json:"run_cmd"`
	TestCmd *[]string `json:"test_cmd"`
	Ext     *string   `json:"ext"`
	TestExt *string   `json:"test_ext"`
}

// @Summary Add a new language driver
// @Tags runners
// @Accept json
// @Param request body AddDriverReq true "Driver config"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runners [post]
func (h *Handler) addRunner(w http.ResponseWriter, r *http.Request) {
	var req AddDriverReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Lang == "" || len(req.Driver.RunCmd) == 0 || req.Driver.Ext == "" {
		h.writeError(w, http.StatusBadRequest, "lang, driver.run_cmd and driver.ext are required")
		return
	}
	if h.runner.HasDriver(req.Lang) {
		h.writeError(w, http.StatusConflict, "driver already exists, use PATCH /runners/{lang} to update")
		return
	}
	bin := req.Driver.RunCmd[0]
	if _, err := exec.LookPath(bin); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("binary %q not found in PATH", bin))
		return
	}
	h.runner.AddDriver(req.Lang, req.Driver)
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Partially update an existing language driver
// @Tags runners
// @Accept json
// @Param lang path string true "Language key (e.g. go)"
// @Param request body PatchDriverReq true "Fields to update"
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

	var req PatchDriverReq
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
	if req.Ext != nil {
		d.Ext = *req.Ext
	}
	if req.TestExt != nil {
		d.TestExt = *req.TestExt
	}

	h.runner.AddDriver(lang, d)
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Delete a language driver
// @Tags runners
// @Param lang path string true "Language key (e.g. go)"
// @Success 204
// @Router /runners/{lang} [delete]
func (h *Handler) deleteRunner(w http.ResponseWriter, r *http.Request) {
	lang := chi.URLParam(r, "lang")
	h.runner.RemoveDriver(lang)
	_ = os.RemoveAll(filepath.Join(h.runnersDir, lang))
	w.WriteHeader(http.StatusNoContent)
}
