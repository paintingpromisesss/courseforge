package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func TestCourseForgeProvider_RealCourses(t *testing.T) {
	coursesDir := filepath.Join("..", "..", "courses")
	if _, err := os.Stat(coursesDir); err != nil {
		t.Skip("courses dir not found, skipping integration test")
	}

	tmpDir, err := os.MkdirTemp("", "mcp-prov-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_submissions.db")
	db, err := repo.NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer db.Close()

	subRepo := repo.NewSubmissionRepository(db)
	progRepo := repo.NewFileProgressRepository(coursesDir)
	r := runner.New()

	prov, err := mcp.NewCourseForgeProvider(coursesDir, progRepo, subRepo, r)
	if err != nil {
		t.Fatalf("new courseforge provider: %v", err)
	}

	ctx := context.Background()
	courses, err := prov.ListCourses(ctx)
	if err != nil {
		t.Fatalf("list courses: %v", err)
	}

	if len(courses) == 0 {
		t.Fatal("expected courses to be loaded")
	}

	// 1. Task Details
	details, err := prov.GetTaskDetails(ctx, "go-interview", "golang-strings-1")
	if err != nil {
		t.Fatalf("get task details: %v", err)
	}
	if details.Title != "Разворот строки" {
		t.Errorf("unexpected task title: %s", details.Title)
	}
	if len(details.Languages) == 0 || details.Languages[0] != "go" {
		t.Errorf("expected go language, got %v", details.Languages)
	}

	// 2. Statement
	statement, err := prov.GetTaskStatement(ctx, "go-interview", "golang-strings-1")
	if err != nil || len(statement) == 0 {
		t.Fatalf("failed to read statement: %v", err)
	}

	// 3. Template
	tplName, tplCode, err := prov.GetTaskTemplate(ctx, "go-interview", "golang-strings-1", "go")
	if err != nil || len(tplCode) == 0 {
		t.Fatalf("failed to read template: %v", err)
	}
	if tplName != "template.go" {
		t.Errorf("unexpected template name: %s", tplName)
	}

	// 4. Tests
	testName, testCode, err := prov.GetTaskTests(ctx, "go-interview", "golang-strings-1", "go")
	if err != nil || len(testCode) == 0 {
		t.Fatalf("failed to read tests: %v", err)
	}
	if testName != "solution_test.go" {
		t.Errorf("unexpected test file name: %s", testName)
	}

	// 5. Solution
	solName, solCode, err := prov.GetTaskSolution(ctx, "go-interview", "golang-strings-1", "go")
	if err != nil || len(solCode) == 0 {
		t.Fatalf("failed to read solution: %v", err)
	}
	if solName != "solution.go" {
		t.Errorf("unexpected solution file name: %s", solName)
	}

	// 6. RunSolution with reference solution
	res, err := prov.RunSolution(ctx, mcp.RunSolutionRequest{
		CourseSlug:     "go-interview",
		TaskSlug:       "golang-strings-1",
		Language:       "go",
		Code:           solCode,
		SaveSubmission: true,
	})
	if err != nil {
		t.Fatalf("run solution: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d, stderr: %s, stdout: %s", res.ExitCode, res.Stderr, res.Stdout)
	}
	if !strings.Contains(res.Stdout, "PASS") {
		t.Errorf("expected tests to pass, stdout: %s", res.Stdout)
	}
	if res.SubmissionID <= 0 {
		t.Errorf("expected submission id to be generated, got %d", res.SubmissionID)
	}

	// 7. Check submissions list
	subs, err := prov.ListSubmissions(ctx, "go-interview", "golang-strings-1", 10)
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(subs) == 0 {
		t.Fatalf("expected at least 1 submission saved")
	}
	if subs[0].ID != res.SubmissionID {
		t.Errorf("expected submission id %d, got %d", res.SubmissionID, subs[0].ID)
	}

	// 8. Check last submission
	lastSub, err := prov.GetLastSubmission(ctx, "go-interview", "golang-strings-1")
	if err != nil || lastSub == nil {
		t.Fatalf("failed to get last submission: %v", err)
	}
	if lastSub.ID != res.SubmissionID {
		t.Errorf("expected last submission id %d, got %d", res.SubmissionID, lastSub.ID)
	}
}
