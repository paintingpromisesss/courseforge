package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)


// @Summary Get MCP server configuration and status
// @Tags mcp
// @Produce json
// @Success 200 {object} dto.MCPStatusResp
// @Router /mcp/config [get]
func (h *Handler) getMCPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.mcpConfigRepo.Get(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Resolve absolute courses and data dir defaults if relative
	coursesDir := cfg.CoursesDir
	if coursesDir == "" || coursesDir == "./courses" {
		coursesDir = h.coursesDir
	}
	dataDir := cfg.DataDir
	if dataDir == "" || dataDir == "./data" {
		dataDir = h.dataDir
	}

	coursesDir = resolveExistingDir(coursesDir)
	dataDir = resolveExistingDir(dataDir)

	// Detect binary command and args
	command, args, available := findMCPCommand(coursesDir, dataDir)

	// SSE URL on current host
	host := r.Host
	if host == "" {
		host = "127.0.0.1:8080"
	}
	sseURL := fmt.Sprintf("http://%s/api/mcp/sse", host)

	resp := dto.MCPStatusResp{
		Enabled:    cfg.Enabled,
		Transport:  cfg.Transport,
		Host:       cfg.Host,
		Port:       cfg.Port,
		CoursesDir: coursesDir,
		DataDir:    dataDir,
		BinaryPath: command,
		Command:    command,
		Args:       args,
		SSEURL:     sseURL,
		Platform:   runtime.GOOS,
		ToolsCount: 9,
		Available:  available,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// @Summary Update MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param request body dto.MCPConfigReq true "MCP Configuration"
// @Success 200 {object} dto.MCPStatusResp
// @Router /mcp/config [patch]
func (h *Handler) patchMCPConfig(w http.ResponseWriter, r *http.Request) {
	var req domain.MCPConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing, _ := h.mcpConfigRepo.Get(r.Context())
	if existing != nil {
		if req.Transport == "" {
			req.Transport = existing.Transport
		}
		if req.Host == "" {
			req.Host = existing.Host
		}
		if req.Port == 0 {
			req.Port = existing.Port
		}
		if req.CoursesDir == "" {
			req.CoursesDir = existing.CoursesDir
		}
		if req.DataDir == "" {
			req.DataDir = existing.DataDir
		}
	}

	if err := h.mcpConfigRepo.Save(r.Context(), &req); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, req)
}

func (h *Handler) handleMCPSSE(w http.ResponseWriter, r *http.Request) {
	if !h.isMCPEnabled(r.Context()) {
		h.writeError(w, http.StatusForbidden, "MCP server is disabled in CourseForge settings")
		return
	}
	if h.sseServer == nil {
		h.writeError(w, http.StatusServiceUnavailable, "MCP server is not configured")
		return
	}
	h.sseServer.SSEHandler().ServeHTTP(w, r)
}

func (h *Handler) handleMCPMessage(w http.ResponseWriter, r *http.Request) {
	if !h.isMCPEnabled(r.Context()) {
		h.writeError(w, http.StatusForbidden, "MCP server is disabled in CourseForge settings")
		return
	}
	if h.sseServer == nil {
		h.writeError(w, http.StatusServiceUnavailable, "MCP server is not configured")
		return
	}
	h.sseServer.MessageHandler().ServeHTTP(w, r)
}

func (h *Handler) getSessionManager() mcp.SessionManager {
	if h.mcpServer != nil && h.mcpServer.Session() != nil {
		return h.mcpServer.Session()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fallbackSession == nil {
		h.fallbackSession, _ = mcp.NewFileSessionManager(filepath.Join(h.dataDir, "mcp_session.json"))
	}
	return h.fallbackSession
}

// @Summary Get active task context for MCP
// @Tags mcp
// @Produce json
// @Success 200 {object} dto.MCPActiveTaskResp
// @Router /mcp/active-task [get]
func (h *Handler) getMCPActiveTask(w http.ResponseWriter, r *http.Request) {
	sm := h.getSessionManager()
	if sm == nil {
		h.writeJSON(w, http.StatusOK, dto.MCPActiveTaskResp{})
		return
	}
	active, err := sm.GetActiveTask(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if active == nil {
		h.writeJSON(w, http.StatusOK, dto.MCPActiveTaskResp{})
		return
	}
	h.writeJSON(w, http.StatusOK, dto.MCPActiveTaskResp{
		CourseSlug: active.CourseSlug,
		TaskSlug:   active.TaskSlug,
		Language:   active.Language,
		UpdatedAt:  active.UpdatedAt.Format(time.RFC3339),
	})
}

// @Summary Set active task context for MCP
// @Tags mcp
// @Accept json
// @Produce json
// @Param request body dto.MCPActiveTaskReq true "Active Task Context"
// @Success 200 {object} dto.MCPActiveTaskResp
// @Router /mcp/active-task [put]
func (h *Handler) putMCPActiveTask(w http.ResponseWriter, r *http.Request) {
	var req dto.MCPActiveTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CourseSlug == "" || req.TaskSlug == "" {
		h.writeError(w, http.StatusBadRequest, "course_slug and task_slug are required")
		return
	}

	sm := h.getSessionManager()
	if sm == nil {
		h.writeError(w, http.StatusInternalServerError, "session manager not available")
		return
	}

	active, err := sm.SetActiveTask(r.Context(), req.CourseSlug, req.TaskSlug, req.Language)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, dto.MCPActiveTaskResp{
		CourseSlug: active.CourseSlug,
		TaskSlug:   active.TaskSlug,
		Language:   active.Language,
		UpdatedAt:  active.UpdatedAt.Format(time.RFC3339),
	})
}

// @Summary Clear active task context for MCP
// @Tags mcp
// @Produce json
// @Success 204 "No Content"
// @Router /mcp/active-task [delete]
func (h *Handler) deleteMCPActiveTask(w http.ResponseWriter, r *http.Request) {
	sm := h.getSessionManager()
	if sm != nil {
		_ = sm.ClearActiveTask(r.Context())
	}
	w.WriteHeader(http.StatusNoContent)
}


func (h *Handler) isMCPEnabled(ctx context.Context) bool {
	if h.mcpConfigRepo == nil {
		return true
	}
	cfg, err := h.mcpConfigRepo.Get(ctx)
	if err != nil || cfg == nil {
		return true
	}
	return cfg.Enabled
}

func resolveExistingDir(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
	}
	parentCandidate := filepath.Join("..", dir)
	if fi, err := os.Stat(parentCandidate); err == nil && fi.IsDir() {
		if abs, err := filepath.Abs(parentCandidate); err == nil {
			return abs
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func findMCPCommand(coursesDir, dataDir string) (string, []string, bool) {
	exeExt := ""
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}

	args := []string{
		"mcp",
		"--courses-dir=" + coursesDir,
		"--data-dir=" + dataDir,
	}

	// 1. Check current running executable
	if exe, err := os.Executable(); err == nil {
		cleanExe := filepath.Clean(exe)
		isTemp := strings.Contains(cleanExe, "go-build") ||
			strings.Contains(cleanExe, "\\Temp\\") ||
			strings.Contains(cleanExe, "/tmp/")

		if !isTemp {
			base := strings.ToLower(filepath.Base(cleanExe))
			if strings.HasPrefix(base, "courseforge") {
				return cleanExe, args, true
			}
		}
	}

	// 2. Check for compiled courseforge binary in bin/ or ../bin/
	candidates := []string{
		filepath.Join("bin", "courseforge"+exeExt),
		filepath.Join("..", "bin", "courseforge"+exeExt),
	}

	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
				return abs, args, true
			}
		}
	}

	// 3. Check system PATH for courseforge
	if p, err := exec.LookPath("courseforge" + exeExt); err == nil {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		return abs, args, true
	}

	// Fallback: standard command name without path
	return "courseforge" + exeExt, args, false
}
