package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func TestStdioEndToEnd_FullProtocolCycle(t *testing.T) {
	coursesDir := filepath.Join("..", "..", "courses")
	if _, err := os.Stat(coursesDir); err != nil {
		t.Skip("courses dir not found, skipping e2e test")
	}

	tmpDir, err := os.MkdirTemp("", "mcp-e2e-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "e2e_submissions.db")
	db, err := repo.NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer db.Close()

	subRepo := repo.NewSubmissionRepository(db)
	progRepo := repo.NewFileProgressRepository(coursesDir)
	r := runner.New()

	prov, err := mcp.NewCourseForgeProvider(coursesDir, tmpDir, progRepo, subRepo, r)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	stateFile := filepath.Join(tmpDir, "session.json")
	session, err := mcp.NewFileSessionManager(stateFile)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	mcpSrv, err := mcp.NewServer(mcp.Config{
		Name:    "courseforge-e2e",
		Version: "1.0.0",
	}, prov, session)
	if err != nil {
		t.Fatalf("new mcp server: %v", err)
	}

	// Create STDIO pipes: client writes to stdinPipeWriter, server reads from stdinPipeReader
	// server writes to stdoutPipeWriter, client reads from stdoutPipeReader
	stdinPipeReader, stdinPipeWriter := io.Pipe()
	stdoutPipeReader, stdoutPipeWriter := io.Pipe()
	defer stdinPipeWriter.Close()
	defer stdoutPipeWriter.Close()

	stdioServer := server.NewStdioServer(mcpSrv.MCPServer())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- stdioServer.Listen(ctx, stdinPipeReader, stdoutPipeWriter)
	}()

	scanner := bufio.NewScanner(stdoutPipeReader)
	reqID := 0

	sendAndReceive := func(method string, params map[string]any) jsonRPCResponse {
		reqID++
		req := jsonRPCRequest{
			JSONRPC: "2.0",
			ID:      reqID,
			Method:  method,
			Params:  params,
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		data = append(data, '\n')

		if _, err := stdinPipeWriter.Write(data); err != nil {
			t.Fatalf("write request: %v", err)
		}

		if !scanner.Scan() {
			t.Fatalf("expected response line, scanner err: %v", scanner.Err())
		}

		line := scanner.Bytes()
		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("unmarshal response %q: %v", string(line), err)
		}
		if resp.Error != nil {
			t.Fatalf("JSON-RPC error in method %s: %s (code %d)", method, resp.Error.Message, resp.Error.Code)
		}
		return resp
	}

	// 1. Initialize
	initResp := sendAndReceive("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-e2e-agent",
			"version": "1.0.0",
		},
	})
	if len(initResp.Result) == 0 {
		t.Fatal("empty initialize result")
	}

	// 2. tools/list
	toolsListResp := sendAndReceive("tools/list", nil)
	var toolsData struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(toolsListResp.Result, &toolsData); err != nil {
		t.Fatalf("unmarshal tools/list: %v", err)
	}
	if len(toolsData.Tools) < 9 {
		t.Fatalf("expected at least 9 tools, got %d", len(toolsData.Tools))
	}

	// 3. tools/call: list_courses
	listCoursesResp := sendAndReceive("tools/call", map[string]any{
		"name":      "list_courses",
		"arguments": map[string]any{},
	})
	if len(listCoursesResp.Result) == 0 {
		t.Fatal("empty list_courses result")
	}

	// 4. tools/call: set_active_task_context
	setActiveResp := sendAndReceive("tools/call", map[string]any{
		"name": "set_active_task_context",
		"arguments": map[string]any{
			"course_slug": "go-interview",
			"task_slug":   "golang-strings-1",
			"language":    "go",
		},
	})
	if len(setActiveResp.Result) == 0 {
		t.Fatal("empty set_active_task_context result")
	}

	// 5. tools/call: get_current_task
	currentTaskResp := sendAndReceive("tools/call", map[string]any{
		"name":      "get_current_task",
		"arguments": map[string]any{},
	})
	var currentTaskResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(currentTaskResp.Result, &currentTaskResult); err != nil {
		t.Fatalf("unmarshal get_current_task: %v", err)
	}
	if len(currentTaskResult.Content) == 0 || !strings.Contains(currentTaskResult.Content[0].Text, "golang-strings-1") {
		t.Fatalf("get_current_task did not return active task: %+v", currentTaskResult)
	}

	// 6. tools/call: get_task_solution
	solResp := sendAndReceive("tools/call", map[string]any{
		"name": "get_task_solution",
		"arguments": map[string]any{
			"course_slug": "go-interview",
			"task_slug":   "golang-strings-1",
			"language":    "go",
		},
	})
	var solContent struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(solResp.Result, &solContent); err != nil || len(solContent.Content) == 0 {
		t.Fatalf("unmarshal solution response: %v", err)
	}

	var parsedSol struct {
		Filename string `json:"filename"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal([]byte(solContent.Content[0].Text), &parsedSol); err != nil || parsedSol.Code == "" {
		t.Fatalf("failed to parse solution content: %v", err)
	}

	// 7. tools/call: run_solution
	runResp := sendAndReceive("tools/call", map[string]any{
		"name": "run_solution",
		"arguments": map[string]any{
			"course_slug":     "go-interview",
			"task_slug":       "golang-strings-1",
			"language":        "go",
			"code":            parsedSol.Code,
			"save_submission": true,
		},
	})
	var runResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(runResp.Result, &runResult); err != nil || len(runResult.Content) == 0 {
		t.Fatalf("unmarshal run_solution: %v", err)
	}
	if !strings.Contains(runResult.Content[0].Text, "PASS") {
		t.Fatalf("run_solution expected PASS, got: %s", runResult.Content[0].Text)
	}

	// 8. tools/call: list_submissions
	subsResp := sendAndReceive("tools/call", map[string]any{
		"name": "list_submissions",
		"arguments": map[string]any{
			"course_slug": "go-interview",
			"task_slug":   "golang-strings-1",
		},
	})
	var subsResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(subsResp.Result, &subsResult); err != nil || len(subsResult.Content) == 0 {
		t.Fatalf("unmarshal list_submissions: %v", err)
	}
	if !strings.Contains(subsResult.Content[0].Text, "submissions") {
		t.Fatalf("expected submissions list: %s", subsResult.Content[0].Text)
	}

	// 9. resources/read: courseforge://active-task
	resActiveResp := sendAndReceive("resources/read", map[string]any{
		"uri": "courseforge://active-task",
	})
	if len(resActiveResp.Result) == 0 {
		t.Fatal("empty active-task resource result")
	}

	// 10. resources/read: courseforge://courses
	resCoursesResp := sendAndReceive("resources/read", map[string]any{
		"uri": "courseforge://courses",
	})
	if len(resCoursesResp.Result) == 0 {
		t.Fatal("empty courses resource result")
	}

	// Clean shutdown
	cancel()
	stdinPipeWriter.Close()
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		fmt.Println("server shutdown timed out")
	}
}
