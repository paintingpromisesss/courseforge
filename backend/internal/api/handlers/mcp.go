package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
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

	absCourses, err := filepath.Abs(coursesDir)
	if err == nil {
		coursesDir = absCourses
	}
	absData, err := filepath.Abs(dataDir)
	if err == nil {
		dataDir = absData
	}

	// Detect binary path
	binaryPath := findMCPBinary()
	available := binaryPath != ""

	resp := dto.MCPStatusResp{
		Enabled:    cfg.Enabled,
		Transport:  cfg.Transport,
		Host:       cfg.Host,
		Port:       cfg.Port,
		CoursesDir: coursesDir,
		DataDir:    dataDir,
		BinaryPath: binaryPath,
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

func findMCPBinary() string {
	binName := "courseforge-mcp"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	// 1. Check in system PATH
	if p, err := exec.LookPath(binName); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	// 2. Check in bin/ relative to CWD
	candidates := []string{
		filepath.Join("bin", binName),
		filepath.Join("..", "bin", binName),
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, binName),
			filepath.Join(exeDir, "..", "bin", binName),
		)
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}

	return binName
}
