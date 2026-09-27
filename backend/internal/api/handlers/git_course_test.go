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
)

// doGitReq routes a request through chi so {courseSlug} URL params resolve.
func doGitReq(t *testing.T, h *Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/courses/{courseSlug}/git/branches", h.getGitBranches)
	r.Post("/courses/{courseSlug}/git/checkout", h.postGitCheckout)
	r.Post("/courses/{courseSlug}/git/pull", h.postGitPull)
	r.Get("/courses/{courseSlug}/git/status", h.getGitStatus)

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

// addBranch commits a mutation of course.yaml on a new branch in repoDir.
func addBranch(t *testing.T, repoDir, branch, yamlContent string) {
	t.Helper()
	gitRun(t, repoDir, "checkout", "-B", branch)
	if err := os.WriteFile(filepath.Join(repoDir, "course.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, "add", "-A")
	gitRun(t, repoDir, "commit", "-m", "branch "+branch)
}

func courseYAML(title, slug string) string {
	return "schema_version: 1\nslug: " + slug + "\ntitle: " + title + "\nlanguage: ru\ntracks:\n  - week-1\n"
}

func setupImportedCourse(t *testing.T) (*Handler, string) {
	t.Helper()
	gitAvailable(t)
	repoDir := makeCourseRepo(t)
	h := newGitTestHandler(t)
	w := postGitImport(t, h, `{"url":`+jsonStr(repoDir)+`}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("import status = %d: %s", w.Code, w.Body)
	}
	return h, repoDir
}

func TestGitBranchesAndCheckout(t *testing.T) {
	h, repoDir := setupImportedCourse(t)
	addBranch(t, repoDir, "dev", courseYAML("Go Dev", "go-interview"))
	addBranch(t, repoDir, "broken", courseYAML("Broken", "wrong-slug"))
	gitRun(t, repoDir, "checkout", "main")

	// GET branches: lists all, current = main, source recorded.
	w := doGitReq(t, h, http.MethodGet, "/courses/go-interview/git/branches", "")
	if w.Code != http.StatusOK {
		t.Fatalf("branches status = %d: %s", w.Code, w.Body)
	}
	var br dto.GitBranchesResp
	if err := json.Unmarshal(w.Body.Bytes(), &br); err != nil {
		t.Fatal(err)
	}
	if br.Current != "main" {
		t.Fatalf("current = %q", br.Current)
	}
	want := map[string]bool{"main": false, "dev": false, "broken": false}
	for _, b := range br.Branches {
		want[b] = true
	}
	for b, seen := range want {
		if !seen {
			t.Fatalf("branch %q missing from %v", b, br.Branches)
		}
	}
	if br.Source == nil || br.Source.Repo != repoDir {
		t.Fatalf("source = %+v", br.Source)
	}

	// Checkout dev: registered course title changes, source updated.
	w = doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/checkout", `{"branch":"dev"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("checkout status = %d: %s", w.Code, w.Body)
	}
	var resp dto.GitImportResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Branch != "dev" || resp.Commit == "" {
		t.Fatalf("resp = %+v", resp)
	}
	c := h.getCourseBySlug("go-interview")
	if c == nil || c.Title != "Go Dev" {
		t.Fatalf("course after checkout = %+v", c)
	}
	sources, _ := h.sources.All(context.Background())
	if sources["go-interview"].Branch != "dev" {
		t.Fatalf("source branch = %q", sources["go-interview"].Branch)
	}

	// Dirty tree without force -> 409.
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	theory := filepath.Join(courseDir, "week-1", "slices", "01-intro", "theory.md")
	if err := os.WriteFile(theory, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	w = doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/checkout", `{"branch":"main"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("dirty checkout status = %d, want 409: %s", w.Code, w.Body)
	}

	// With force -> 200, local edit gone.
	w = doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/checkout", `{"branch":"main","force":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("forced checkout status = %d: %s", w.Code, w.Body)
	}
	if c := h.getCourseBySlug("go-interview"); c.Title != "Go" {
		t.Fatalf("title after force checkout = %q", c.Title)
	}

	// GET status reflects the clean tree on main.
	w = doGitReq(t, h, http.MethodGet, "/courses/go-interview/git/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	var st dto.GitStatusResp
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || st.Dirty || st.Commit == "" {
		t.Fatalf("status = %+v", st)
	}
}

func TestCheckoutInvalidBranchRollsBack(t *testing.T) {
	h, repoDir := setupImportedCourse(t)
	addBranch(t, repoDir, "broken", courseYAML("Broken", "wrong-slug"))
	gitRun(t, repoDir, "checkout", "main")

	w := doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/checkout", `{"branch":"broken"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body)
	}
	if h.getCourseBySlug("go-interview") == nil {
		t.Fatal("course lost after failed checkout")
	}
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	branch, err := h.gitSvc.CurrentBranch(context.Background(), courseDir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Fatalf("branch after rollback = %q, want main", branch)
	}
	data, err := os.ReadFile(filepath.Join(courseDir, "course.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "title: Go") {
		t.Fatalf("course.yaml not rolled back: %s", data)
	}
}

func TestGitPull(t *testing.T) {
	h, repoDir := setupImportedCourse(t)

	// New commit on main in the source repo.
	addBranch(t, repoDir, "main", courseYAML("Go Pulled", "go-interview"))

	w := doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/pull", "")
	if w.Code != http.StatusOK {
		t.Fatalf("pull status = %d: %s", w.Code, w.Body)
	}
	if c := h.getCourseBySlug("go-interview"); c.Title != "Go Pulled" {
		t.Fatalf("title after pull = %q", c.Title)
	}
	sources, _ := h.sources.All(context.Background())
	if sources["go-interview"].Commit == "" {
		t.Fatal("source commit not updated")
	}

	// Dirty tree without force -> 409.
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	theory := filepath.Join(courseDir, "week-1", "slices", "01-intro", "theory.md")
	if err := os.WriteFile(theory, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	w = doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/pull", "")
	if w.Code != http.StatusConflict {
		t.Fatalf("dirty pull status = %d, want 409", w.Code)
	}
	// Force pull discards the local edit.
	w = doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/pull", `{"force":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("forced pull status = %d: %s", w.Code, w.Body)
	}
	data, _ := os.ReadFile(theory)
	if got := strings.TrimSpace(string(data)); got != "# Intro" {
		t.Fatalf("theory.md after force pull = %q", data)
	}
}

func TestGitPullMerge(t *testing.T) {
	h, repoDir := setupImportedCourse(t)
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	theory := filepath.Join(courseDir, "week-1", "slices", "01-intro", "theory.md")
	plain := filepath.Join(courseDir, "week-1", "track.yaml")

	// local edits in two files; upstream will change one of them
	if err := os.WriteFile(theory, []byte("my intro"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plain, []byte("slug: week-1\ntitle: My Week\ntopics:\n  - slices\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// upstream updates the course (theory.md untouched by them → merges fine)
	addBranch(t, repoDir, "main", courseYAML("Go Merged", "go-interview"))

	w := doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/pull", `{"mode":"merge"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("merge pull status = %d: %s", w.Code, w.Body)
	}
	// upstream title applied
	if c := h.getCourseBySlug("go-interview"); c.Title != "Go Merged" {
		t.Fatalf("title after merge pull = %q", c.Title)
	}
	// local edits preserved
	data, _ := os.ReadFile(theory)
	if string(data) != "my intro" {
		t.Fatalf("theory.md lost after merge pull: %q", data)
	}
	data, _ = os.ReadFile(plain)
	if !strings.Contains(string(data), "My Week") {
		t.Fatalf("track.yaml lost after merge pull: %q", data)
	}
}

func TestGitPullMergeConflictKeepsStash(t *testing.T) {
	h, repoDir := setupImportedCourse(t)
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	yamlPath := filepath.Join(courseDir, "course.yaml")

	// local edit to the same line upstream will change
	if err := os.WriteFile(yamlPath, []byte(courseYAML("My Local Title", "go-interview")), 0644); err != nil {
		t.Fatal(err)
	}
	addBranch(t, repoDir, "main", courseYAML("Go Upstream", "go-interview"))

	w := doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/pull", `{"mode":"merge"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("conflicting merge pull status = %d: %s", w.Code, w.Body)
	}
	// course rolled to upstream content and still parses
	data, _ := os.ReadFile(yamlPath)
	if !strings.Contains(string(data), "Go Upstream") {
		t.Fatalf("course.yaml not on upstream after conflict: %q", data)
	}
	if h.getCourseBySlug("go-interview") == nil {
		t.Fatal("course lost after conflicting merge pull")
	}
	// local edits recoverable from the stash
	stash, err := h.gitSvc.StashList(context.Background(), courseDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stash, "cf-pull") {
		t.Fatalf("stash entry missing: %q", stash)
	}
}

func TestGitPullModeForceDiscards(t *testing.T) {
	h, repoDir := setupImportedCourse(t)
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	theory := filepath.Join(courseDir, "week-1", "slices", "01-intro", "theory.md")

	// local edit + upstream update
	if err := os.WriteFile(theory, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	addBranch(t, repoDir, "main", courseYAML("Go Force", "go-interview"))

	// mode:force (the UI's «Затереть») must discard the edit, not 409
	w := doGitReq(t, h, http.MethodPost, "/courses/go-interview/git/pull", `{"mode":"force"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("mode=force pull status = %d: %s", w.Code, w.Body)
	}
	data, _ := os.ReadFile(theory)
	if got := strings.TrimSpace(string(data)); got != "# Intro" {
		t.Fatalf("theory.md after mode=force = %q", data)
	}
	if c := h.getCourseBySlug("go-interview"); c.Title != "Go Force" {
		t.Fatalf("title after mode=force = %q", c.Title)
	}
}

func TestGitEndpointsRejectNonGitCourse(t *testing.T) {
	h := newGitTestHandler(t)

	// Unknown slug -> 404.
	w := doGitReq(t, h, http.MethodGet, "/courses/nope/git/branches", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown slug status = %d, want 404", w.Code)
	}

	// Registered course without a git source -> 404.
	h.mu.Lock()
	h.courses["plain"] = &domain.Course{Slug: "plain", Dir: "plain", Title: "Plain"}
	h.mu.Unlock()
	w = doGitReq(t, h, http.MethodGet, "/courses/plain/git/branches", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("no-source status = %d, want 404: %s", w.Code, w.Body)
	}
	w = doGitReq(t, h, http.MethodPost, "/courses/plain/git/checkout", `{"branch":"dev"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no-source checkout status = %d, want 404", w.Code)
	}
}

// doAttachReq routes an attach request; body carries all inputs.
func doAttachReq(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/git/attach", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.gitAttach(w, req)
	return w
}

// setupPlainCourse creates a local course directory without .git and registers it.
func setupPlainCourse(t *testing.T, h *Handler) string {
	t.Helper()
	courseDir := filepath.Join(h.coursesDir, "go-interview")
	writeCourseRepoFiles(t, courseDir)
	if _, err := h.loadAndRegisterCourse("go-interview"); err != nil {
		t.Fatal(err)
	}
	return courseDir
}

func TestGitAttach(t *testing.T) {
	gitAvailable(t)
	repoDir := makeCourseRepo(t)
	h := newGitTestHandler(t)
	courseDir := setupPlainCourse(t, h)

	// a local edit proves attach never touches course files
	localYAML := courseYAML("My Local Version", "go-interview")
	if err := os.WriteFile(filepath.Join(courseDir, "course.yaml"), []byte(localYAML), 0644); err != nil {
		t.Fatal(err)
	}

	w := doAttachReq(t, h, `{"url":`+jsonStr(repoDir)+`,"slug":"go-interview"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attach status = %d: %s", w.Code, w.Body)
	}
	var resp dto.GitImportResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Slug != "go-interview" || resp.Branch != "main" || resp.Commit == "" {
		t.Fatalf("attach resp = %+v", resp)
	}

	// .git now exists in the course dir; course files untouched
	if _, err := os.Stat(filepath.Join(courseDir, ".git")); err != nil {
		t.Fatalf(".git not moved into course: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(courseDir, "course.yaml"))
	if string(data) != localYAML {
		t.Fatalf("attach modified course files: %q", data)
	}
	// source recorded
	sources, _ := h.sources.All(context.Background())
	if sources["go-interview"].Repo != repoDir {
		t.Fatalf("source not recorded: %+v", sources)
	}
	// the local version differs from the remote → dirty
	dirty, err := h.gitSvc.Dirty(context.Background(), courseDir)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatal("local version should be dirty relative to remote after attach")
	}
	// branches endpoint now serves this course
	w = doGitReq(t, h, http.MethodGet, "/courses/go-interview/git/branches", "")
	if w.Code != http.StatusOK {
		t.Fatalf("branches after attach = %d: %s", w.Code, w.Body)
	}
}

func TestGitAttachSlugMismatch(t *testing.T) {
	gitAvailable(t)
	repoDir := makeCourseRepo(t) // slug go-interview
	h := newGitTestHandler(t)
	setupPlainCourse(t, h)

	// a local course with a different slug → the repo's manifest slug doesn't match
	otherDir := filepath.Join(h.coursesDir, "other")
	os.MkdirAll(otherDir, 0755)
	os.WriteFile(filepath.Join(otherDir, "course.yaml"), []byte("slug: other\n"), 0644)
	h.mu.Lock()
	h.courses["other"] = &domain.Course{Slug: "other", Dir: "other", Title: "Other"}
	h.mu.Unlock()

	w := doAttachReq(t, h, `{"url":`+jsonStr(repoDir)+`,"slug":"other"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mismatch status = %d: %s", w.Code, w.Body)
	}
	// nothing happened to the target dir
	if _, err := os.Stat(filepath.Join(otherDir, ".git")); err == nil {
		t.Fatal(".git leaked into mismatched course")
	}
}

func TestGitAttachAlreadyImported(t *testing.T) {
	gitAvailable(t)
	h, repoDir := setupImportedCourse(t)
	w := doAttachReq(t, h, `{"url":`+jsonStr(repoDir)+`,"slug":"go-interview"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("already-imported attach status = %d: %s", w.Code, w.Body)
	}
}
