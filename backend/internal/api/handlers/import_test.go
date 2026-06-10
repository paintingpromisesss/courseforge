package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/api/dto"
	"github.com/paintingpromisesss/courseforge/internal/domain"
)

func TestImportCourseUsesDeclaredSlugDestination(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "copied-course")
	writeTestCourse(t, sourceDir, "real-course")

	coursesDir := t.TempDir()
	h := &Handler{
		coursesDir: coursesDir,
		courses:    map[string]*domain.Course{},
	}

	body, err := json.Marshal(dto.ImportCourseReq{Path: sourceDir})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/courses/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.importCourse(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp dto.SlugResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Slug != "real-course" {
		t.Fatalf("slug = %q, want %q", resp.Slug, "real-course")
	}

	if _, err := os.Stat(filepath.Join(coursesDir, "real-course")); err != nil {
		t.Fatalf("expected course to be installed under declared slug: %v", err)
	}
	if _, err := os.Stat(filepath.Join(coursesDir, "copied-course")); !os.IsNotExist(err) {
		t.Fatalf("unexpected import under source folder name, err = %v", err)
	}
	if h.getCourseBySlug("real-course") == nil {
		t.Fatal("expected course to be registered by declared slug")
	}
}

func TestImportCourseRejectsExistingDeclaredSlug(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "copied-course")
	writeTestCourse(t, sourceDir, "real-course")

	coursesDir := t.TempDir()
	h := &Handler{
		coursesDir: coursesDir,
		courses: map[string]*domain.Course{
			"real-course": {Slug: "real-course"},
		},
	}

	body, err := json.Marshal(dto.ImportCourseReq{Path: sourceDir})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/courses/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.importCourse(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != `course slug "real-course" already exists` {
		t.Fatalf("unexpected response body: %v", resp)
	}
	if _, err := os.Stat(filepath.Join(coursesDir, "real-course")); !os.IsNotExist(err) {
		t.Fatalf("conflicting import should not create destination, err = %v", err)
	}
}

func writeTestCourse(t *testing.T, rootDir, slug string) {
	t.Helper()

	files := map[string]string{
		"course.yaml": strings.Join([]string{
			"schema_version: 1",
			"slug: " + slug,
			"title: Test Course",
			"language: ru",
			"tracks:",
			"  - track-1",
			"",
		}, "\n"),
		"track-1/track.yaml": strings.Join([]string{
			"slug: track-1",
			"title: Track 1",
			"topics:",
			"  - topic-1",
			"",
		}, "\n"),
		"track-1/topic-1/topic.yaml": strings.Join([]string{
			"slug: topic-1",
			"title: Topic 1",
			"units:",
			"  - unit-1",
			"",
		}, "\n"),
		"track-1/topic-1/unit-1/unit.yaml": strings.Join([]string{
			"slug: unit-1",
			"title: Unit 1",
			"tasks:",
			"  - task-1",
			"",
		}, "\n"),
		"track-1/topic-1/unit-1/task-1/task.yaml": strings.Join([]string{
			"slug: task-1",
			"title: Task 1",
			"statement: statement.md",
			"languages:",
			"  go:",
			"    template: template.go",
			"",
		}, "\n"),
		"track-1/topic-1/unit-1/task-1/statement.md":   "# Statement\n",
		"track-1/topic-1/unit-1/task-1/go/template.go": "package main\n",
	}

	for name, content := range files {
		path := filepath.Join(rootDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
