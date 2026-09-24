package handlers

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/paintingpromisesss/courseforge/internal/updater"
)

type VersionResponse struct {
	Version   string               `json:"version"`
	OS        string               `json:"os"`
	Arch      string               `json:"arch"`
	GoVersion string               `json:"go_version"`
	IsDev     bool                 `json:"is_dev"`
	Check     *updater.CheckResult `json:"check,omitempty"`
	Status    updater.Status       `json:"status"`
}

// getVersion returns current version, platform details, and cached/current update status.
func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	if h.updater == nil {
		h.writeJSON(w, http.StatusOK, VersionResponse{
			Version:   "dev",
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			GoVersion: runtime.Version(),
			IsDev:     true,
			Status:    updater.Status{State: "idle"},
		})
		return
	}

	ver := h.updater.CurrentVersion()
	isDev := ver == "dev" || strings.HasPrefix(ver, "dev-")

	h.writeJSON(w, http.StatusOK, VersionResponse{
		Version:   ver,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		IsDev:     isDev,
		Check:     h.updater.LastCheck(),
		Status:    h.updater.GetStatus(),
	})
}

// checkVersion forces a live check against GitHub Releases for new updates.
func (h *Handler) checkVersion(w http.ResponseWriter, r *http.Request) {
	if h.updater == nil {
		h.writeError(w, http.StatusServiceUnavailable, "updater not initialized")
		return
	}

	res, err := h.updater.Check(r.Context(), true)
	if err != nil {
		h.writeError(w, http.StatusBadGateway, "failed to check updates: "+err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, res)
}

// updateVersion triggers the background download and atomic binary replacement.
func (h *Handler) updateVersion(w http.ResponseWriter, r *http.Request) {
	if h.updater == nil {
		h.writeError(w, http.StatusServiceUnavailable, "updater not initialized")
		return
	}

	status := h.updater.GetStatus()
	if status.State == "downloading" {
		h.writeJSON(w, http.StatusConflict, status)
		return
	}

	if err := h.updater.StartUpdate(r.Context()); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusAccepted, h.updater.GetStatus())
}

// restartVersion triggers a self-restart of the CourseForge binary.
func (h *Handler) restartVersion(w http.ResponseWriter, r *http.Request) {
	if h.updater == nil {
		h.writeError(w, http.StatusServiceUnavailable, "updater not initialized")
		return
	}

	if err := h.updater.Restart(); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to restart: "+err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}
