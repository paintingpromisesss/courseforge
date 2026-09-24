package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

type mockProvider struct {
	courses     []mcp.CourseSummary
	taskDetails map[string]*mcp.TaskDetails
	templates   map[string]string
	solutions   map[string]string
	tests       map[string]string
	submissions map[string][]mcp.SubmissionItem
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		courses: []mcp.CourseSummary{
			{
				Slug:           "go-interview",
				Title:          "Go Interview Tasks",
				Description:    "Practice tasks",
				Language:       "go",
				TotalTasks:     5,
				CompletedTasks: 2,
			},
		},
		taskDetails: map[string]*mcp.TaskDetails{
			"go-interview/merge-sorted": {
				CourseSlug:  "go-interview",
				TaskSlug:    "merge-sorted",
				Title:       "Merge Sorted Arrays",
				Statement:   "# Merge Sorted Arrays\nGiven two sorted arrays...",
				Languages:   []string{"go", "python"},
				Limits:      mcp.TaskLimits{TimeoutSec: 5, MemoryMB: 128},
				IsCompleted: false,
			},
		},
		templates: map[string]string{
			"go-interview/merge-sorted/go": "package solution\nfunc Merge() {}",
		},
		solutions: map[string]string{
			"go-interview/merge-sorted/go": "package solution\nfunc Merge() { /* optimal */ }",
		},
		tests: map[string]string{
			"go-interview/merge-sorted/go": "package solution\nimport \"testing\"\nfunc TestMerge(t *testing.T) {}",
		},
		submissions: map[string][]mcp.SubmissionItem{
			"go-interview/merge-sorted": {
				{
					ID:          1,
					Language:    "go",
					Code:        "package solution...",
					Stdout:      "PASS",
					Stderr:      "",
					ExitCode:    0,
					PassedTests: 3,
					TotalTests:  3,
					DurationMs:  45,
					TimedOut:    false,
					CreatedAt:   time.Now().UTC(),
				},
			},
		},
	}
}

func (m *mockProvider) ListCourses(ctx context.Context) ([]mcp.CourseSummary, error) {
	return m.courses, nil
}

func (m *mockProvider) GetTaskDetails(ctx context.Context, courseSlug, taskSlug string) (*mcp.TaskDetails, error) {
	key := courseSlug + "/" + taskSlug
	t, ok := m.taskDetails[key]
	if !ok {
		return nil, errors.New("task not found")
	}
	return t, nil
}

func (m *mockProvider) GetTaskStatement(ctx context.Context, courseSlug, taskSlug string) (string, error) {
	details, err := m.GetTaskDetails(ctx, courseSlug, taskSlug)
	if err != nil {
		return "", err
	}
	return details.Statement, nil
}

func (m *mockProvider) GetTaskTemplate(ctx context.Context, courseSlug, taskSlug, language string) (string, string, error) {
	key := courseSlug + "/" + taskSlug + "/" + language
	code, ok := m.templates[key]
	if !ok {
		return "", "", errors.New("template not found")
	}
	return "solution.go", code, nil
}

func (m *mockProvider) GetTaskSolution(ctx context.Context, courseSlug, taskSlug, language string) (string, string, error) {
	key := courseSlug + "/" + taskSlug + "/" + language
	code, ok := m.solutions[key]
	if !ok {
		return "", "", errors.New("solution not found")
	}
	return "solution.go", code, nil
}

func (m *mockProvider) GetTaskTests(ctx context.Context, courseSlug, taskSlug, language string) (string, string, error) {
	key := courseSlug + "/" + taskSlug + "/" + language
	code, ok := m.tests[key]
	if !ok {
		return "", "", errors.New("tests not found")
	}
	return "solution_test.go", code, nil
}

func (m *mockProvider) ListSubmissions(ctx context.Context, courseSlug, taskSlug string, limit int) ([]mcp.SubmissionItem, error) {
	key := courseSlug + "/" + taskSlug
	return m.submissions[key], nil
}

func (m *mockProvider) GetLastSubmission(ctx context.Context, courseSlug, taskSlug string) (*mcp.SubmissionItem, error) {
	subs, err := m.ListSubmissions(ctx, courseSlug, taskSlug, 1)
	if err != nil || len(subs) == 0 {
		return nil, err
	}
	return &subs[0], nil
}

func (m *mockProvider) RunSolution(ctx context.Context, req mcp.RunSolutionRequest) (*mcp.RunSolutionResult, error) {
	return &mcp.RunSolutionResult{
		Stdout:       "PASS\nok  solution 0.05s",
		Stderr:       "",
		ExitCode:     0,
		PassedTests:  5,
		TotalTests:   5,
		DurationMs:   50,
		TimedOut:     false,
		SubmissionID: 42,
	}, nil
}

func (m *mockProvider) ReloadCourses(ctx context.Context) error {
	return nil
}

func (m *mockProvider) CreateTask(ctx context.Context, req mcp.CreateTaskRequest) (*mcp.TaskDetails, error) {
	return &mcp.TaskDetails{
		CourseSlug: req.CourseSlug,
		TaskSlug:   req.TaskSlug,
		Title:      req.Title,
		Statement:  req.Statement,
		Languages:  []string{req.Language},
	}, nil
}

func (m *mockProvider) EditTaskStatement(ctx context.Context, courseSlug, taskSlug, content string) error {
	return nil
}

func (m *mockProvider) EditTaskCode(ctx context.Context, courseSlug, taskSlug, language, fileType, content string) (string, error) {
	return "template.go", nil
}

func (m *mockProvider) UpdateTaskMetadata(ctx context.Context, req mcp.UpdateTaskMetadataRequest) (*mcp.TaskDetails, error) {
	return &mcp.TaskDetails{
		CourseSlug: req.CourseSlug,
		TaskSlug:   req.TaskSlug,
	}, nil
}

func (m *mockProvider) EditUnitTheory(ctx context.Context, courseSlug, unitSlug, content string) error {
	return nil
}

func (m *mockProvider) SaveNote(ctx context.Context, courseSlug, unitSlug, content, mode string) error {
	return nil
}

func (m *mockProvider) GetNote(ctx context.Context, courseSlug, unitSlug string) (string, bool, error) {
	return "# Note", true, nil
}

func setupTestServer(t *testing.T) (*mcp.Server, *mockProvider, mcp.SessionManager) {
	t.Helper()
	prov := newMockProvider()
	session, err := mcp.NewFileSessionManager("")
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	srv, err := mcp.NewServer(mcp.Config{
		Name:    "courseforge-mcp-test",
		Version: "1.0.0",
	}, prov, session)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}
	return srv, prov, session
}

func TestServer_ToolsList(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	tools := srv.MCPServer().ListTools()

	expectedTools := []string{
		"get_current_task",
		"set_active_task_context",
		"list_courses",
		"get_task_details",
		"get_task_template",
		"get_task_solution",
		"get_task_tests",
		"list_submissions",
		"run_solution",
		"create_task",
		"edit_task_statement",
		"edit_task_code",
		"update_task_metadata",
		"edit_unit_theory",
		"save_note",
		"get_note",
	}

	for _, name := range expectedTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestServer_JSONRPC_InitializeAndCallTools(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ctx := context.Background()

	// 1. Initialize request
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`
	resp := srv.MCPServer().HandleMessage(ctx, json.RawMessage(initReq))
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal init response: %v", err)
	}
	if len(respBytes) == 0 {
		t.Fatal("empty response to initialize")
	}

	// 2. Call list_courses
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(listReq))
	respBytes, _ = json.Marshal(resp)
	if !json.Valid(respBytes) {
		t.Fatalf("invalid json response: %s", string(respBytes))
	}

	// 3. Call set_active_task_context
	setReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"set_active_task_context","arguments":{"course_slug":"go-interview","task_slug":"merge-sorted","language":"go"}}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(setReq))
	respBytes, _ = json.Marshal(resp)
	if string(respBytes) == "" {
		t.Fatalf("empty set_active_task_context response")
	}

	// 4. Call get_current_task
	getReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_current_task","arguments":{}}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(getReq))
	respBytes, _ = json.Marshal(resp)
	if string(respBytes) == "" {
		t.Fatalf("empty get_current_task response")
	}

	// 5. Call run_solution
	runReq := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"run_solution","arguments":{"course_slug":"go-interview","task_slug":"merge-sorted","language":"go","code":"package solution\nfunc Merge() {}","save_submission":true}}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(runReq))
	respBytes, _ = json.Marshal(resp)
	if string(respBytes) == "" {
		t.Fatalf("empty run_solution response")
	}
}

func TestServer_JSONRPC_ReadResources(t *testing.T) {
	srv, _, session := setupTestServer(t)
	ctx := context.Background()

	// Set active task in session
	_, err := session.SetActiveTask(ctx, "go-interview", "merge-sorted", "go")
	if err != nil {
		t.Fatalf("failed to set active task: %v", err)
	}

	// 1. Read courseforge://active-task
	readActiveReq := `{"jsonrpc":"2.0","id":10,"method":"resources/read","params":{"uri":"courseforge://active-task"}}`
	resp := srv.MCPServer().HandleMessage(ctx, json.RawMessage(readActiveReq))
	respBytes, err := json.Marshal(resp)
	if err != nil || len(respBytes) == 0 {
		t.Fatalf("failed reading active-task resource: %v", err)
	}

	// 2. Read courseforge://courses
	readCoursesReq := `{"jsonrpc":"2.0","id":11,"method":"resources/read","params":{"uri":"courseforge://courses"}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(readCoursesReq))
	respBytes, err = json.Marshal(resp)
	if err != nil || len(respBytes) == 0 {
		t.Fatalf("failed reading courses resource: %v", err)
	}

	// 3. Read statement template
	readStmtReq := `{"jsonrpc":"2.0","id":12,"method":"resources/read","params":{"uri":"courseforge://courses/go-interview/tasks/merge-sorted/statement"}}`
	resp = srv.MCPServer().HandleMessage(ctx, json.RawMessage(readStmtReq))
	respBytes, err = json.Marshal(resp)
	if err != nil || len(respBytes) == 0 {
		t.Fatalf("failed reading statement resource: %v", err)
	}
}
