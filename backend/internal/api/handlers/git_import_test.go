package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func gitAvailable(t *testing.T) {
	t.Helper()
	if exec.Command("git", "--version").Run() != nil {
		t.Skip("git not installed")
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitRunCapture(t, dir, args...)
}

// gitRunCapture runs git and returns its combined output, failing the test
// on a non-zero exit.
func gitRunCapture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// newGitTestHandler builds a Handler with only the dependencies git import needs.
func newGitTestHandler(t *testing.T) *Handler {
	t.Helper()
	tempDir := t.TempDir()
	coursesDir := filepath.Join(tempDir, "courses")
	if err := os.MkdirAll(coursesDir, 0755); err != nil {
		t.Fatal(err)
	}
	h := New(coursesDir, tempDir,
		map[string]*domain.Course{}, map[string]*domain.Catalog{},
		nil, nil, nil, nil, nil, nil, git.NewService(), nil, nil, nil)
	h.sources = repo.NewCourseSourcesRepository(tempDir)
	return h
}

func writeCourseRepoFiles(t *testing.T, dir string) {
	t.Helper()
	w := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	w("course.yaml", `schema_version: 1
slug: go-interview
title: Go
language: ru
tracks:
  - week-1
`)
	w("week-1/track.yaml", `slug: week-1
title: "Week 1"
topics:
  - slices
`)
	w("week-1/slices/topic.yaml", `slug: slices
title: Slices
units:
  - 01-intro
`)
	w("week-1/slices/01-intro/unit.yaml", `slug: 01-intro
title: Intro
theory: theory.md
`)
	w("week-1/slices/01-intro/theory.md", "# Intro\n")
}

// makeCourseRepo creates a committed local git repo with a valid course at its root.
func makeCourseRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeCourseRepoFiles(t, dir)
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

func postGitImport(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/git/import", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.gitImport(w, req)
	return w
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestValidateRepoURL(t *testing.T) {
	good := []string{
		"https://github.com/owner/repo",
		"https://github.com/owner/repo.git",
		"https://github.com/owner/repo/",
		"http://github.com/owner/repo",
	}
	for _, u := range good {
		if _, ok := validateRepoURL(u); !ok {
			t.Errorf("validateRepoURL(%q) = false, want true", u)
		}
	}
	bad := []string{
		"",
		"https://gitlab.com/x/y",
		"https://github.com/owner",
		"https://github.com/owner/repo/extra",
		"git@github.com:owner/repo.git",
		"file:///tmp/x",
		"--upload-pack=x",
		"http://github.com.evil.com/o/r",
	}
	for _, u := range bad {
		if _, ok := validateRepoURL(u); ok {
			t.Errorf("validateRepoURL(%q) = true, want false", u)
		}
	}
	// local existing directories pass (tests + local clones)
	dir := t.TempDir()
	if _, ok := validateRepoURL(dir); !ok {
		t.Errorf("local dir %q rejected", dir)
	}
}

func TestGitImport(t *testing.T) {
	gitAvailable(t)
	repoDir := makeCourseRepo(t)
	h := newGitTestHandler(t)

	w := postGitImport(t, h, `{"url":`+jsonStr(repoDir)+`}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	var resp dto.GitImportResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Slug != "go-interview" {
		t.Fatalf("slug = %q", resp.Slug)
	}
	if resp.Branch != "main" || resp.Commit == "" {
		t.Fatalf("branch/commit = %q/%q", resp.Branch, resp.Commit)
	}
	if _, err := os.Stat(filepath.Join(h.coursesDir, "go-interview", ".git")); err != nil {
		t.Fatal(".git not preserved in imported course")
	}
	if h.getCourseBySlug("go-interview") == nil {
		t.Fatal("course not registered")
	}
	sources, err := h.sources.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sources["go-interview"].Repo != repoDir || sources["go-interview"].Commit != resp.Commit {
		t.Fatalf("source = %+v", sources["go-interview"])
	}

	// Re-import collides -> 409.
	w = postGitImport(t, h, `{"url":`+jsonStr(repoDir)+`}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("re-import status = %d, want 409", w.Code)
	}
}

func TestGitImportRejectsURL(t *testing.T) {
	h := newGitTestHandler(t)
	w := postGitImport(t, h, `{"url":"https://gitlab.com/x/y"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	w = postGitImport(t, h, `{"url":""}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty url status = %d, want 400", w.Code)
	}
}

func TestGitImportRejectsRepoWithoutManifest(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")

	h := newGitTestHandler(t)
	w := postGitImport(t, h, `{"url":`+jsonStr(dir)+`}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body)
	}
}

func TestGitImportAuthFailureClassification(t *testing.T) {
	// classifyCloneError is pure — no network needed.
	cases := map[string]int{
		"remote: HTTP Basic: Access denied\nfatal: Authentication failed for 'https://github.com/o/r'": http.StatusUnauthorized,
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled":           http.StatusUnauthorized,
		"remote: Repository not found.\nfatal: repository 'https://github.com/o/r/' not found":          http.StatusUnauthorized,
		"fatal: unable to access 'https://github.com/o/r/': Could not resolve host: github.com":          http.StatusBadRequest,
	}
	for stderr, want := range cases {
		got := classifyCloneError(&git.Error{Stderr: stderr})
		if got != want {
			t.Errorf("classifyCloneError(%q) = %d, want %d", stderr, got, want)
		}
	}
}

func TestGitImportBatch(t *testing.T) {
	gitAvailable(t)

	repoA := makeCourseRepo(t) // valid course, slug go-interview

	// repo with a broken manifest: slug missing → import fails, clone succeeds
	repoB := t.TempDir()
	os.WriteFile(filepath.Join(repoB, "course.yaml"), []byte("title: no slug\n"), 0644)
	gitRun(t, repoB, "init", "-b", "main")
	gitRun(t, repoB, "add", "-A")
	gitRun(t, repoB, "commit", "-m", "init")

	// repo without any manifest → structure error
	repoC := t.TempDir()
	os.WriteFile(filepath.Join(repoC, "README.md"), []byte("x"), 0644)
	gitRun(t, repoC, "init", "-b", "main")
	gitRun(t, repoC, "add", "-A")
	gitRun(t, repoC, "commit", "-m", "init")

	h := newGitTestHandler(t)
	body := `{"urls":[` +
		jsonStr(repoA) + `,` +
		jsonStr(repoB) + `,` +
		jsonStr(repoC) + `,` +
		jsonStr("https://gitlab.com/x/y") + `]}`
	w := postGitImportBatch(t, h, body)
	if w.Code != http.StatusOK {
		t.Fatalf("batch status = %d: %s", w.Code, w.Body)
	}
	var resp dto.GitImportBatchResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 4 {
		t.Fatalf("results = %+v", resp.Results)
	}

	byURL := map[string]dto.GitImportBatchItem{}
	for _, it := range resp.Results {
		byURL[it.URL] = it
	}

	// valid repo imported
	okA := byURL[repoA]
	if !okA.OK || okA.Slug != "go-interview" || okA.Error != "" {
		t.Fatalf("repoA result = %+v", okA)
	}
	// no slug → import-level error, not clone error
	badB := byURL[repoB]
	if badB.OK || badB.Slug != "" || badB.Error == "" {
		t.Fatalf("repoB result = %+v", badB)
	}
	// no manifest → structure error
	badC := byURL[repoC]
	if badC.OK || !strings.Contains(badC.Error, "manifest") && !strings.Contains(badC.Error, "course.yaml") {
		t.Fatalf("repoC result = %+v", badC)
	}
	// invalid URL → validation error
	badD := byURL["https://gitlab.com/x/y"]
	if badD.OK || badD.Error == "" {
		t.Fatalf("repoD result = %+v", badD)
	}

	// only the valid course is on disk and registered
	if h.getCourseBySlug("go-interview") == nil {
		t.Fatal("valid course not imported")
	}
	sources, _ := h.sources.All(context.Background())
	if sources["go-interview"].Repo != repoA {
		t.Fatalf("source not recorded: %+v", sources)
	}
}

func TestGitImportBatchEmptyList(t *testing.T) {
	h := newGitTestHandler(t)
	w := postGitImportBatch(t, h, `{"urls":[]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty batch status = %d", w.Code)
	}
}

func TestGitImportBatchSkipsDuplicatesInList(t *testing.T) {
	gitAvailable(t)
	repo := makeCourseRepo(t)
	h := newGitTestHandler(t)
	body := `{"urls":[` + jsonStr(repo) + `,` + jsonStr(repo) + `]}`
	w := postGitImportBatch(t, h, body)
	if w.Code != http.StatusOK {
		t.Fatalf("batch status = %d: %s", w.Code, w.Body)
	}
	var resp dto.GitImportBatchResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	imported, dups := 0, 0
	for _, it := range resp.Results {
		if it.OK {
			imported++
		} else {
			dups++
		}
	}
	if imported != 1 || dups != 1 {
		t.Fatalf("imported=%d dups=%d: %+v", imported, dups, resp.Results)
	}
}

func postGitImportBatch(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/git/import/batch", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.gitImportBatch(w, req)
	return w
}
