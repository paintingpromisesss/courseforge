package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func setupTestCourse(t *testing.T) (coursesDir, dataDir string, cleanup func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "cf-content-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	cDir := filepath.Join(tmpDir, "courses")
	dDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(cDir, 0755); err != nil {
		t.Fatalf("mkdir courses: %v", err)
	}
	if err := os.MkdirAll(dDir, 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	// Create valid course hierarchy
	courseDir := filepath.Join(cDir, "demo-course")
	trackDir := filepath.Join(courseDir, "track-1")
	topicDir := filepath.Join(trackDir, "topic-1")
	unitDir := filepath.Join(topicDir, "unit-1")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		t.Fatalf("mkdir unit: %v", err)
	}

	files := map[string]string{
		filepath.Join(courseDir, "course.yaml"): `
schema_version: 1
slug: demo-course
title: Demo Course
language: ru
tracks:
  - track-1
`,
		filepath.Join(trackDir, "track.yaml"): `
slug: track-1
title: Track 1
topics:
  - topic-1
`,
		filepath.Join(topicDir, "topic.yaml"): `
slug: topic-1
title: Topic 1
units:
  - unit-1
`,
		filepath.Join(unitDir, "unit.yaml"): `
slug: unit-1
title: Unit 1
theory: theory.md
`,
		filepath.Join(unitDir, "theory.md"): "# Unit 1 Theory\nInitial theory content.\n",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0644); err != nil {
			t.Fatalf("write file %s: %v", path, err)
		}
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir)
	}
	return cDir, dDir, cleanup
}

func TestContent_Notes(t *testing.T) {
	cDir, dDir, cleanup := setupTestCourse(t)
	defer cleanup()

	r := runner.New()
	progRepo := repo.NewFileProgressRepository(cDir)
	prov, err := mcp.NewCourseForgeProvider(cDir, dDir, progRepo, nil, r)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ctx := context.Background()

	// 1. Initial check - note does not exist
	content, hasNote, err := prov.GetNote(ctx, "demo-course", "unit-1")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if hasNote || content != "" {
		t.Fatalf("expected no note, got hasNote=%v, content=%q", hasNote, content)
	}

	// 2. Save note (overwrite)
	noteText1 := "# Мой конспект\nИзучил основы слайсов."
	if err := prov.SaveNote(ctx, "demo-course", "unit-1", noteText1, "overwrite"); err != nil {
		t.Fatalf("save note overwrite: %v", err)
	}

	content, hasNote, err = prov.GetNote(ctx, "demo-course", "unit-1")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if !hasNote || content != noteText1 {
		t.Fatalf("expected note %q, got %q", noteText1, content)
	}

	// 3. Save note (append)
	noteText2 := "Дополнительно: емкость удваивается."
	if err := prov.SaveNote(ctx, "demo-course", "unit-1", noteText2, "append"); err != nil {
		t.Fatalf("save note append: %v", err)
	}

	content, hasNote, err = prov.GetNote(ctx, "demo-course", "unit-1")
	if err != nil {
		t.Fatalf("get note after append: %v", err)
	}
	expectedAppended := noteText1 + "\n\n" + noteText2
	if !hasNote || content != expectedAppended {
		t.Fatalf("expected %q, got %q", expectedAppended, content)
	}

	// 4. Invalid slug
	if err := prov.SaveNote(ctx, "../escape", "unit-1", "bad", "overwrite"); err == nil {
		t.Fatal("expected error for path traversal slug, got nil")
	}

	// 5. Invalid mode
	if err := prov.SaveNote(ctx, "demo-course", "unit-1", "bad", "random_mode"); err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}

	// 6. Delete note
	if err := prov.DeleteNote(ctx, "demo-course", "unit-1"); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	content, hasNote, err = prov.GetNote(ctx, "demo-course", "unit-1")
	if err != nil {
		t.Fatalf("get note after delete: %v", err)
	}
	if hasNote || content != "" {
		t.Fatalf("expected note to be deleted, got hasNote=%v, content=%q", hasNote, content)
	}
}

func TestContent_EditUnitTheory(t *testing.T) {
	cDir, dDir, cleanup := setupTestCourse(t)
	defer cleanup()

	r := runner.New()
	progRepo := repo.NewFileProgressRepository(cDir)
	prov, err := mcp.NewCourseForgeProvider(cDir, dDir, progRepo, nil, r)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ctx := context.Background()

	newTheory := "# Новая теория\nОбновленный материал лекции."
	if err := prov.EditUnitTheory(ctx, "demo-course", "unit-1", newTheory); err != nil {
		t.Fatalf("edit theory: %v", err)
	}

	theoryPath := filepath.Join(cDir, "demo-course", "track-1", "topic-1", "unit-1", "theory.md")
	data, err := os.ReadFile(theoryPath)
	if err != nil {
		t.Fatalf("read theory.md: %v", err)
	}
	if string(data) != newTheory {
		t.Fatalf("theory content mismatch: expected %q, got %q", newTheory, string(data))
	}
}

func TestContent_CreateTaskAndEdits(t *testing.T) {
	cDir, dDir, cleanup := setupTestCourse(t)
	defer cleanup()

	r := runner.New()
	progRepo := repo.NewFileProgressRepository(cDir)
	prov, err := mcp.NewCourseForgeProvider(cDir, dDir, progRepo, nil, r)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ctx := context.Background()

	// 1. Create task
	req := mcp.CreateTaskRequest{
		CourseSlug:   "demo-course",
		UnitSlug:     "unit-1",
		TaskSlug:     "task-sum",
		Title:        "Сумма чисел",
		Statement:    "# Сумма чисел\nНапишите функцию Sum(a, b int) int.",
		Language:     "go",
		TemplateCode: "package solution\nfunc Sum(a, b int) int { return 0 }",
		TestsCode:    "package solution\nimport \"testing\"\nfunc TestSum(t *testing.T) { if Sum(1, 2) != 3 { t.Fail() } }",
		SolutionCode: "package solution\nfunc Sum(a, b int) int { return a + b }",
		Difficulty:   2,
		Tags:         []string{"math", "basics"},
		TimeoutSec:   5,
		MemoryMB:     128,
	}

	details, err := prov.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if details.TaskSlug != "task-sum" || details.Title != "Сумма чисел" || details.Difficulty != 2 {
		t.Fatalf("unexpected task details: %+v", details)
	}

	// Verify unit.yaml contains task-sum
	unitYAMLPath := filepath.Join(cDir, "demo-course", "track-1", "topic-1", "unit-1", "unit.yaml")
	unitYAMLBytes, _ := os.ReadFile(unitYAMLPath)
	if !strings.Contains(string(unitYAMLBytes), "task-sum") {
		t.Fatalf("unit.yaml does not contain task-sum:\n%s", string(unitYAMLBytes))
	}

	// 2. Edit statement
	newStatement := "# Новое условие\nСложите два числа."
	if err := prov.EditTaskStatement(ctx, "demo-course", "task-sum", newStatement); err != nil {
		t.Fatalf("edit statement: %v", err)
	}

	stmt, err := prov.GetTaskStatement(ctx, "demo-course", "task-sum")
	if err != nil {
		t.Fatalf("get statement: %v", err)
	}
	if stmt != newStatement {
		t.Fatalf("expected statement %q, got %q", newStatement, stmt)
	}

	// 3. Edit template code
	newTemplate := "package solution\n// TODO: return a + b\nfunc Sum(a, b int) int { return 0 }"
	filename, err := prov.EditTaskCode(ctx, "demo-course", "task-sum", "go", "template", newTemplate)
	if err != nil {
		t.Fatalf("edit code: %v", err)
	}
	if filename != "template.go" {
		t.Fatalf("expected template.go, got %s", filename)
	}

	_, templateCode, err := prov.GetTaskTemplate(ctx, "demo-course", "task-sum", "go")
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if templateCode != newTemplate {
		t.Fatalf("template mismatch: expected %q, got %q", newTemplate, templateCode)
	}

	// 4. Update task metadata
	newTitle := "Сумма двух целых чисел"
	newDiff := 3
	newTags := []string{"math", "arithmetic"}
	updatedDetails, err := prov.UpdateTaskMetadata(ctx, mcp.UpdateTaskMetadataRequest{
		CourseSlug: "demo-course",
		TaskSlug:   "task-sum",
		Title:      &newTitle,
		Difficulty: &newDiff,
		Tags:       &newTags,
	})
	if err != nil {
		t.Fatalf("update task metadata: %v", err)
	}
	if updatedDetails.Title != newTitle || updatedDetails.Difficulty != 3 || len(updatedDetails.Tags) != 2 {
		t.Fatalf("unexpected updated details: %+v", updatedDetails)
	}

	// 5. Delete task
	if err := prov.DeleteTask(ctx, "demo-course", "task-sum"); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	// Verify task details return error (not found)
	if _, err := prov.GetTaskDetails(ctx, "demo-course", "task-sum"); err == nil {
		t.Fatal("expected error getting deleted task details, got nil")
	}

	// Verify unit.yaml no longer contains task-sum
	unitYAMLBytes, _ = os.ReadFile(unitYAMLPath)
	if strings.Contains(string(unitYAMLBytes), "task-sum") {
		t.Fatalf("unit.yaml still contains task-sum after deletion:\n%s", string(unitYAMLBytes))
	}
}

func TestServer_ContentToolsRegistrationAndExecution(t *testing.T) {
	cDir, dDir, cleanup := setupTestCourse(t)
	defer cleanup()

	r := runner.New()
	progRepo := repo.NewFileProgressRepository(cDir)
	prov, err := mcp.NewCourseForgeProvider(cDir, dDir, progRepo, nil, r)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	sess, err := mcp.NewFileSessionManager(filepath.Join(dDir, "session.json"))
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	srv, err := mcp.NewServer(mcp.Config{
		Name:    "test-server",
		Version: "1.0.0",
	}, prov, sess)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	mcpSrv := srv.MCPServer()

	// Verify all 7 tools are registered
	expectedTools := []string{
		"create_task",
		"edit_task_statement",
		"edit_task_code",
		"update_task_metadata",
		"edit_unit_theory",
		"save_note",
		"get_note",
		"delete_task",
		"delete_note",
	}

	tools := mcpSrv.ListTools()
	for _, name := range expectedTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %s is not registered", name)
		}
	}

	ctx := context.Background()

	// 1. Initialize session via JSON-RPC
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}`
	_ = mcpSrv.HandleMessage(ctx, json.RawMessage(initReq))

	// 2. Call save_note via JSON-RPC
	saveNoteReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"save_note","arguments":{"course_slug":"demo-course","unit_slug":"unit-1","content":"# Konspekt\nVse rabotaet!","mode":"overwrite"}}}`
	resp := mcpSrv.HandleMessage(ctx, json.RawMessage(saveNoteReq))
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal save_note response: %v", err)
	}
	if strings.Contains(string(respBytes), `"isError":true`) {
		t.Fatalf("save_note returned error: %s", string(respBytes))
	}

	// 3. Call get_note via JSON-RPC
	getNoteReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_note","arguments":{"course_slug":"demo-course","unit_slug":"unit-1"}}}`
	resp = mcpSrv.HandleMessage(ctx, json.RawMessage(getNoteReq))
	respBytes, err = json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal get_note response: %v", err)
	}
	if strings.Contains(string(respBytes), `"isError":true`) {
		t.Fatalf("get_note returned error: %s", string(respBytes))
	}
	if !strings.Contains(string(respBytes), "Vse rabotaet!") {
		t.Fatalf("expected get_note result to contain note text, got: %s", string(respBytes))
	}
}
