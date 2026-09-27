package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	csync "github.com/paintingpromisesss/courseforge/internal/infrastructure/sync"
)

// newSyncTestHandler builds a Handler wired with a sync Engine whose mirror
// lives in tempDir; cloud is a local non-bare repo acting as the remote.
func newSyncTestHandler(t *testing.T) (*Handler, *csync.Engine, string, string) {
	t.Helper()
	gitAvailable(t)
	tempDir := t.TempDir()
	coursesDir := filepath.Join(tempDir, "courses")
	if err := os.MkdirAll(coursesDir, 0755); err != nil {
		t.Fatal(err)
	}
	cloud := filepath.Join(tempDir, "cloud")
	if err := os.MkdirAll(cloud, 0755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloud, "init", "-b", "main")
	gitRun(t, cloud, "config", "receive.denyCurrentBranch", "ignore")
	gitRun(t, cloud, "config", "core.autocrlf", "false")

	h := New(coursesDir, tempDir,
		map[string]*domain.Course{}, map[string]*domain.Catalog{},
		nil, nil, nil, nil, nil, nil, git.NewService(), nil, nil, nil)
	h.sources = repo.NewCourseSourcesRepository(tempDir)
	cfgRepo := repo.NewSyncConfigRepository(tempDir)
	engine := csync.NewEngine(h.gitSvc, cfgRepo, h.sources, coursesDir, tempDir)
	h.SetSyncEngine(engine)
	return h, engine, cloud, tempDir
}

// doSyncReq routes through chi so URL params (/sync/history/{commit}) resolve.
func doSyncReq(t *testing.T, h *Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/sync/config", h.getSyncConfig)
	r.Patch("/sync/config", h.patchSyncConfig)
	r.Post("/sync/push", h.postSyncPush)
	r.Post("/sync/pull", h.postSyncPull)
	r.Get("/sync/status", h.getSyncStatus)
	r.Get("/sync/history", h.getSyncHistory)
	r.Get("/sync/history/{commit}", h.getSyncCommitFiles)
	r.Post("/sync/rollback", h.postSyncRollback)
	r.Post("/sync/restore-imports", h.postSyncRestoreImports)

	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// configureSync points the engine at cloud via the PATCH endpoint.
func configureSync(t *testing.T, h *Handler, cloud string) {
	t.Helper()
	body := `{"remote_url":` + jsonStr(cloud) + `,"branch":"main","enabled":true}`
	w := doSyncReq(t, h, http.MethodPatch, "/sync/config", body)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /sync/config: %d %s", w.Code, w.Body)
	}
}

func writeSyncCourse(t *testing.T, dir, slug, content string) {
	t.Helper()
	p := filepath.Join(dir, slug)
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "course.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncConfigGetPatch(t *testing.T) {
	h, _, _, _ := newSyncTestHandler(t)

	// absent config → defaults (branch main), remote empty
	w := doSyncReq(t, h, http.MethodGet, "/sync/config", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", w.Code, w.Body)
	}
	var cfg dto.SyncConfigResp
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Branch != "main" || cfg.Enabled || cfg.RemoteURL != "" {
		t.Fatalf("defaults: %+v", cfg)
	}

	// PATCH persists
	w = doSyncReq(t, h, http.MethodPatch, "/sync/config",
		`{"remote_url":"https://github.com/me/vault","branch":"sync","enabled":true,"exclude":["x"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", w.Code, w.Body)
	}
	w = doSyncReq(t, h, http.MethodGet, "/sync/config", "")
	cfg = dto.SyncConfigResp{}
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.RemoteURL != "https://github.com/me/vault" || cfg.Branch != "sync" || !cfg.Enabled {
		t.Fatalf("persisted: %+v", cfg)
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != "x" {
		t.Fatalf("exclude: %+v", cfg.Exclude)
	}

	// PATCH with nil fields keeps the rest unchanged
	w = doSyncReq(t, h, http.MethodPatch, "/sync/config", `{"enabled":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH nil: %d %s", w.Code, w.Body)
	}
	w = doSyncReq(t, h, http.MethodGet, "/sync/config", "")
	cfg = dto.SyncConfigResp{}
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if cfg.RemoteURL != "https://github.com/me/vault" || cfg.Enabled {
		t.Fatalf("partial patch broke config: %+v", cfg)
	}
}

func TestSyncPushPullStatusEndpoints(t *testing.T) {
	h, _, cloud, tempDir := newSyncTestHandler(t)
	coursesDir := filepath.Join(tempDir, "courses")
	configureSync(t, h, cloud)

	writeSyncCourse(t, coursesDir, "c1", "one\n")

	w := doSyncReq(t, h, http.MethodPost, "/sync/push", "")
	if w.Code != http.StatusOK {
		t.Fatalf("push: %d %s", w.Code, w.Body)
	}
	var st dto.SyncStatusResp
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Configured || !st.Enabled || st.Syncing || st.Commit == "" || st.Branch != "main" {
		t.Fatalf("status after push: %+v", st)
	}

	// history shows the sync commit
	w = doSyncReq(t, h, http.MethodGet, "/sync/history?limit=10", "")
	if w.Code != http.StatusOK {
		t.Fatalf("history: %d %s", w.Code, w.Body)
	}
	var hist []dto.SyncHistoryItem
	if err := json.Unmarshal(w.Body.Bytes(), &hist); err != nil {
		t.Fatal(err)
	}
	if len(hist) < 1 || !strings.HasPrefix(hist[0].Subject, "sync:") || hist[0].Commit == "" {
		t.Fatalf("history: %+v", hist)
	}

	// files of the commit
	w = doSyncReq(t, h, http.MethodGet, "/sync/history/"+hist[0].Commit, "")
	if w.Code != http.StatusOK {
		t.Fatalf("commit files: %d %s", w.Code, w.Body)
	}
	var files dto.SyncCommitFilesResp
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files.Files) == 0 {
		t.Fatalf("no files: %+v", files)
	}

	// non-hex commit → 400
	w = doSyncReq(t, h, http.MethodGet, "/sync/history/not-a-commit", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad commit: %d %s", w.Code, w.Body)
	}

	// pull is a no-op round trip here
	w = doSyncReq(t, h, http.MethodPost, "/sync/pull", "")
	if w.Code != http.StatusOK {
		t.Fatalf("pull: %d %s", w.Code, w.Body)
	}
}

func TestSyncPushNotConfigured(t *testing.T) {
	h, _, _, _ := newSyncTestHandler(t)
	w := doSyncReq(t, h, http.MethodPost, "/sync/push", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("push without config: %d %s", w.Code, w.Body)
	}
	w = doSyncReq(t, h, http.MethodPost, "/sync/pull", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("pull without config: %d %s", w.Code, w.Body)
	}
}

func TestSyncRollbackRestoresOldContent(t *testing.T) {
	h, _, cloud, tempDir := newSyncTestHandler(t)
	coursesDir := filepath.Join(tempDir, "courses")
	configureSync(t, h, cloud)

	writeSyncCourse(t, coursesDir, "c1", "version one\n")
	if w := doSyncReq(t, h, http.MethodPost, "/sync/push", ""); w.Code != http.StatusOK {
		t.Fatalf("push1: %d %s", w.Code, w.Body)
	}

	w := doSyncReq(t, h, http.MethodGet, "/sync/history?limit=10", "")
	var hist []dto.SyncHistoryItem
	json.Unmarshal(w.Body.Bytes(), &hist)
	firstCommit := hist[len(hist)-1].Commit

	writeSyncCourse(t, coursesDir, "c1", "version two\n")
	if w := doSyncReq(t, h, http.MethodPost, "/sync/push", ""); w.Code != http.StatusOK {
		t.Fatalf("push2: %d %s", w.Code, w.Body)
	}

	w = doSyncReq(t, h, http.MethodPost, "/sync/rollback", `{"commit":`+jsonStr(firstCommit)+`}`)
	if w.Code != http.StatusOK {
		t.Fatalf("rollback: %d %s", w.Code, w.Body)
	}

	if got, err := os.ReadFile(filepath.Join(coursesDir, "c1", "course.yaml")); err != nil || string(got) != "version one\n" {
		t.Fatalf("rollback content = %q, err %v", got, err)
	}
	w = doSyncReq(t, h, http.MethodGet, "/sync/history?limit=10", "")
	hist = nil
	json.Unmarshal(w.Body.Bytes(), &hist)
	if len(hist) < 3 {
		t.Fatalf("expected 3+ commits after rollback: %d", len(hist))
	}
}

func TestSyncPushRaceRetry(t *testing.T) {
	h, _, cloud, tempDir := newSyncTestHandler(t)
	coursesDir := filepath.Join(tempDir, "courses")
	configureSync(t, h, cloud)

	writeSyncCourse(t, coursesDir, "c1", "device A state\n")
	if w := doSyncReq(t, h, http.MethodPost, "/sync/push", ""); w.Code != http.StatusOK {
		t.Fatalf("push1: %d %s", w.Code, w.Body)
	}

	// device B commits directly to the cloud working tree while A is idle.
	if err := os.WriteFile(filepath.Join(cloud, "foreign.txt"), []byte("from device B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloud, "add", "-A")
	gitRun(t, cloud, "commit", "-m", "foreign change")

	// A pushes again: must realign to the foreign commit (not force-push)
	// and overwrite the snapshot tree with local state.
	writeSyncCourse(t, coursesDir, "c1", "device A state v2\n")
	if w := doSyncReq(t, h, http.MethodPost, "/sync/push", ""); w.Code != http.StatusOK {
		t.Fatalf("push after race: %d %s", w.Code, w.Body)
	}

	if got := cloudShow(t, cloud, "courses/c1/course.yaml"); got != "device A state v2\n" {
		t.Fatalf("cloud content = %q", got)
	}
	out := gitOut(t, cloud, "log", "--oneline")
	if !strings.Contains(out, "foreign change") {
		t.Fatalf("foreign commit lost from history:\n%s", out)
	}
}

func TestSyncRestoreImports(t *testing.T) {
	h, _, cloud, tempDir := newSyncTestHandler(t)
	coursesDir := filepath.Join(tempDir, "courses")
	configureSync(t, h, cloud)

	// source repo whose course slug matches the recorded dir ("imp")
	impRepo := t.TempDir()
	writeCourseRepoFiles(t, impRepo)
	gitRun(t, impRepo, "init", "-b", "main")
	gitRun(t, impRepo, "add", "-A")
	gitRun(t, impRepo, "commit", "-m", "init")
	fixup := filepath.Join(impRepo, "course.yaml")
	if err := os.WriteFile(fixup, []byte(strings.Replace(
		mustReadFile(t, fixup), "slug: go-interview", "slug: imp", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, impRepo, "add", "-A")
	gitRun(t, impRepo, "commit", "-m", "rename slug")
	if err := h.sources.Set(context.Background(), "imp", repo.CourseSource{
		Repo: impRepo, Branch: "main",
	}); err != nil {
		t.Fatal(err)
	}
	writeSyncCourse(t, coursesDir, "plain", "p\n")
	if w := doSyncReq(t, h, http.MethodPost, "/sync/push", ""); w.Code != http.StatusOK {
		t.Fatalf("push: %d %s", w.Code, w.Body)
	}

	// device B: fresh handler+engine, pointed at the same cloud
	h2, _, _, _ := newSyncTestHandler(t)
	w := doSyncReq(t, h2, http.MethodPatch, "/sync/config",
		`{"remote_url":`+jsonStr(cloud)+`,"branch":"main","enabled":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reconfig: %d %s", w.Code, w.Body)
	}
	if w := doSyncReq(t, h2, http.MethodPost, "/sync/pull", ""); w.Code != http.StatusOK {
		t.Fatalf("pull: %d %s", w.Code, w.Body)
	}
	st, err := h2.syncEngine.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(st.PendingImports) != 1 || st.PendingImports[0] != "imp" {
		t.Fatalf("pending imports = %+v", st.PendingImports)
	}

	w = doSyncReq(t, h2, http.MethodPost, "/sync/restore-imports", "")
	if w.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", w.Code, w.Body)
	}
	var resp struct {
		Restored []string `json:"restored"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Restored) != 1 || resp.Restored[0] != "imp" {
		t.Fatalf("restored = %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(h2.coursesDir, "imp", ".git")); err != nil {
		t.Fatalf("import not restored: %v", err)
	}
	if h2.getCourseBySlug("imp") == nil {
		t.Fatal("restored course not registered")
	}
}

func cloudShow(t *testing.T, cloud, path string) string {
	t.Helper()
	return gitOut(t, cloud, "show", "main:"+filepath.ToSlash(path))
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return gitRunCapture(t, dir, args...)
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
