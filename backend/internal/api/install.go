package api

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/runner"
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

// ── handlers ─────────────────────────────────────────────────────────────────

type InstallReq struct {
	Lang    string   `json:"lang"`
	Pkg     string   `json:"pkg,omitempty"`     // apt package name; mutually exclusive with URL
	URL     string   `json:"url,omitempty"`     // archive URL; mutually exclusive with Pkg
	BinPath string   `json:"bin_path"`          // binary name (apt) or relative path in archive
	RunCmd  []string `json:"run_cmd"`           // may use {bin} placeholder
	TestCmd []string `json:"test_cmd"`
	Ext     string   `json:"ext"`
	TestExt string   `json:"test_ext"`
}

func (h *Handler) installRunner(w http.ResponseWriter, r *http.Request) {
	var req InstallReq
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

	h.writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
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
	h.writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"progress": progress,
		"message":  msg,
	})
}

// ── install goroutine ─────────────────────────────────────────────────────────

func (h *Handler) runInstall(req InstallReq, prog *installProgress) {
	if req.Pkg != "" {
		h.runAptInstall(req, prog)
		return
	}
	h.runArchiveInstall(req, prog)
}

func (h *Handler) runAptInstall(req InstallReq, prog *installProgress) {
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

func (h *Handler) runArchiveInstall(req InstallReq, prog *installProgress) {
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

// ── archive helpers ───────────────────────────────────────────────────────────

func archiveFormat(url string) string {
	u := strings.ToLower(url)
	switch {
	case strings.HasSuffix(u, ".tar.gz") || strings.HasSuffix(u, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(u, ".tar.bz2") || strings.HasSuffix(u, ".tbz2"):
		return "tar.bz2"
	case strings.HasSuffix(u, ".zip"):
		return "zip"
	default:
		return ""
	}
}

func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	return extractTar(gz, destDir)
}

func extractTarBz2(r io.Reader, destDir string) error {
	return extractTar(bzip2.NewReader(r), destDir)
}

func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		clean := filepath.Join(destDir, filepath.Clean("/"+hdr.Name)[1:])
		if !strings.HasPrefix(clean, destDir+string(os.PathSeparator)) && clean != destDir {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(clean, 0755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(clean), 0755)
			f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			f.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.Symlink(hdr.Linkname, clean)
		}
	}
	return nil
}

func extractZip(f *os.File, destDir string) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	for _, zf := range zr.File {
		clean := filepath.Join(destDir, filepath.Clean("/"+zf.Name)[1:])
		if !strings.HasPrefix(clean, destDir+string(os.PathSeparator)) && clean != destDir {
			continue
		}
		if zf.FileInfo().IsDir() {
			_ = os.MkdirAll(clean, 0755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(clean), 0755)
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
