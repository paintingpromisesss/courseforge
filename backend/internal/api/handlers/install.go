package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

// ── progress ─────────────────────────────────────────────────────────────────

type installProgress struct {
	mu       sync.Mutex
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message,omitempty"`
}

func (p *installProgress) set(status string, progress int, message string) {
	p.mu.Lock()
	p.Status = status
	p.Progress = progress
	p.Message = message
	p.mu.Unlock()
}

func (p *installProgress) snapshot() (status string, progress int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status, p.Progress, p.Message
}

// ── handlers ───────────────────────────────────────────────────────────────

func (h *Handler) installRunner(w http.ResponseWriter, r *http.Request) {
	var req dto.InstallReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Lang == "" || req.BinPath == "" || len(req.RunCmd) == 0 || req.Ext == "" {
		h.writeError(w, http.StatusBadRequest, "lang, bin_path, run_cmd and ext are required")
		return
	}
	if req.Pkg == "" && req.URL == "" {
		h.writeError(w, http.StatusBadRequest, "pkg or url is required")
		return
	}
	if req.URL != "" && archiveFormat(req.URL) == "" {
		h.writeError(w, http.StatusBadRequest, "unsupported archive format (supported: .tar.gz, .tar.bz2, .zip)")
		return
	}

	prog := &installProgress{}
	if req.Pkg != "" {
		prog.set("installing", 0, "")
	} else {
		prog.set("downloading", 0, "")
	}
	h.installJobs.Store(req.Lang, prog)

	go h.runInstall(req, prog)

	h.writeJSON(w, http.StatusAccepted, dto.StatusResp{Status: "started"})
}

func (h *Handler) getInstallStatus(w http.ResponseWriter, r *http.Request) {
	lang := chi.URLParam(r, "lang")
	val, ok := h.installJobs.Load(lang)
	if !ok {
		h.writeError(w, http.StatusNotFound, "no install job for this language")
		return
	}
	prog := val.(*installProgress)
	status, progress, msg := prog.snapshot()
	h.writeJSON(w, http.StatusOK, dto.InstallStatusResp{
		Status:   status,
		Progress: progress,
		Message:  msg,
	})
}

// ── install goroutine ─────────────────────────────────────────────────────────

func (h *Handler) runInstall(req dto.InstallReq, prog *installProgress) {
	if req.Pkg != "" {
		h.runAptInstall(req, prog)
		return
	}
	h.runArchiveInstall(req, prog)
}

func (h *Handler) runAptInstall(req dto.InstallReq, prog *installProgress) {
	prog.set("installing", 10, "apt-get install -y "+req.Pkg)

	cmd := exec.Command("apt-get", "install", "-y", req.Pkg)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 200 {
			msg = msg[len(msg)-200:]
		}
		prog.set("error", 0, "apt error: "+msg)
		return
	}

	binPath, err := exec.LookPath(req.BinPath)
	if err != nil {
		prog.set("error", 0, "binary not found after install: "+req.BinPath)
		return
	}

	resolveCmd := func(cmds []string) []string {
		out := make([]string, len(cmds))
		for i, s := range cmds {
			out[i] = strings.ReplaceAll(s, "{bin}", binPath)
		}
		return out
	}

	h.runner.AddDriver(req.Lang, runner.LangDriver{
		RunCmd:  resolveCmd(req.RunCmd),
		TestCmd: resolveCmd(req.TestCmd),
		Ext:     req.Ext,
		TestExt: req.TestExt,
	})
	prog.set("done", 100, "")
}

func (h *Handler) runArchiveInstall(req dto.InstallReq, prog *installProgress) {
	destDir := filepath.Join(h.runnersDir, req.Lang)
	format := archiveFormat(req.URL)

	// download
	resp, err := http.Get(req.URL) //nolint:gosec
	if err != nil {
		prog.set("error", 0, "download failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "cf-install-*")
	if err != nil {
		prog.set("error", 0, "temp file error: "+err.Error())
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	total := resp.ContentLength
	downloaded := int64(0)
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmpFile.Write(buf[:n]); werr != nil {
				prog.set("error", 0, "write error: "+werr.Error())
				return
			}
			downloaded += int64(n)
			if total > 0 {
				prog.set("downloading", int(downloaded*100/total), "")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			prog.set("error", 0, "download error: "+err.Error())
			return
		}
	}

	prog.set("extracting", 100, "")

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		prog.set("error", 0, "seek error: "+err.Error())
		return
	}

	tmpExtract, err := os.MkdirTemp("", "cf-extract-*")
	if err != nil {
		prog.set("error", 0, "extract dir error: "+err.Error())
		return
	}
	defer os.RemoveAll(tmpExtract)

	switch format {
	case "tar.gz":
		err = extractTarGz(tmpFile, tmpExtract)
	case "tar.bz2":
		err = extractTarBz2(tmpFile, tmpExtract)
	case "zip":
		err = extractZip(tmpFile, tmpExtract)
	}
	if err != nil {
		prog.set("error", 0, "extract error: "+err.Error())
		return
	}

	entries, err := os.ReadDir(tmpExtract)
	if err != nil || len(entries) == 0 {
		prog.set("error", 0, "unexpected archive structure")
		return
	}

	if err := os.MkdirAll(h.runnersDir, 0755); err != nil {
		prog.set("error", 0, "mkdir error: "+err.Error())
		return
	}
	_ = os.RemoveAll(destDir)

	// single top-level directory → rename it; otherwise move the whole extract dir
	var srcDir string
	if len(entries) == 1 && entries[0].IsDir() {
		srcDir = filepath.Join(tmpExtract, entries[0].Name())
	} else {
		srcDir = tmpExtract
	}
	if err := os.Rename(srcDir, destDir); err != nil {
		prog.set("error", 0, "install error: "+err.Error())
		return
	}

	binPath := filepath.Join(destDir, filepath.FromSlash(req.BinPath))
	_ = os.Chmod(binPath, 0755)

	resolveCmd := func(cmd []string) []string {
		out := make([]string, len(cmd))
		for i, s := range cmd {
			out[i] = strings.ReplaceAll(s, "{bin}", binPath)
		}
		return out
	}

	h.runner.AddDriver(req.Lang, runner.LangDriver{
		RunCmd:  resolveCmd(req.RunCmd),
		TestCmd: resolveCmd(req.TestCmd),
		Ext:     req.Ext,
		TestExt: req.TestExt,
	})
	prog.set("done", 100, "")
}
