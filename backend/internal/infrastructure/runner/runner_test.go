package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_Go_Playground(t *testing.T) {
	r := New()
	res, err := r.Run(context.Background(), RunRequest{
		Language: "go",
		Code: `package main
import "fmt"
func main() { fmt.Println("hello") }`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d stderr: %s", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("stdout %q does not contain 'hello'", res.Stdout)
	}
}

func TestRun_Go_NonZeroExit(t *testing.T) {
	r := New()
	res, err := r.Run(context.Background(), RunRequest{
		Language: "go",
		Code: `package main
import "os"
func main() { os.Exit(2) }`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit, got 0")
	}
}

func TestRun_Timeout(t *testing.T) {
	r := New()
	res, err := r.Run(context.Background(), RunRequest{
		Language: "go",
		Code: `package main
import "time"
func main() { time.Sleep(time.Hour) }`,
		Timeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut {
		t.Fatal("expected TimedOut=true")
	}
}

func TestRun_ContextCancel(t *testing.T) {
	r := New()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	res, err := r.Run(ctx, RunRequest{
		Language: "go",
		Code: `package main
import "time"
func main() { time.Sleep(time.Hour) }`,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Cancellation is not a timeout: the process was killed on ctx.Done, not the deadline.
	if res.TimedOut {
		t.Fatal("expected TimedOut=false for client cancellation")
	}
}

func TestSaveFile_SkipsUnchangedBuiltins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runners.json")
	r := New()
	if err := r.UseFile(path); err != nil {
		t.Fatal(err)
	}

	read := func() map[string]LangDriver {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]LangDriver
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	// untouched built-in go must not be frozen on disk
	if _, ok := read()["go"]; ok {
		t.Fatal("unchanged built-in 'go' should not be persisted")
	}

	// a patched built-in must be persisted so the override survives
	r.AddDriver("go", LangDriver{RunCmd: []string{"go", "run", "{file}"}, Ext: ".go"})
	if _, ok := read()["go"]; !ok {
		t.Fatal("patched 'go' driver should be persisted")
	}
}

func TestPrepareExpandsRelativeFileNames(t *testing.T) {
	// {file}/{testfile} must expand to names relative to the temp dir, not
	// absolute paths: shell-string templates (cmd /c "...", sh -c "...") get
	// them interpolated unquoted, so an absolute temp path containing a space
	// (e.g. a Windows user name with one) would split the command.
	r := New()
	d := LangDriver{
		RunCmd:  []string{"sh", "-c", "node {file}"},
		TestCmd: []string{"sh", "-c", "node {file} && mocha {testfile}"},
		Ext:     ".js",
		TestExt: "_test.js",
	}
	dir, args, err := r.prepare(d, RunRequest{Language: "javascript", Code: "1", TestCode: "2"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if want := "node solution.js && mocha solution_test.js"; args[2] != want {
		t.Errorf("got %q, want %q", args[2], want)
	}
}

func TestRun_UnsupportedLanguage(t *testing.T) {
	r := New()
	_, err := r.Run(context.Background(), RunRequest{Language: "brainfuck", Code: "+++"})
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRun_Go_TestMode_Pass(t *testing.T) {
	r := New()
	res, err := r.Run(context.Background(), RunRequest{
		Language: "go",
		Code: `package main
func Add(a, b int) int { return a + b }
func main() {}`,
		TestCode: `package main
import "testing"
func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("expected 3")
	}
}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("tests failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
}

func TestRun_Go_TestMode_Fail(t *testing.T) {
	r := New()
	res, err := r.Run(context.Background(), RunRequest{
		Language: "go",
		Code: `package main
func Add(a, b int) int { return a - b }
func main() {}`,
		TestCode: `package main
import "testing"
func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("expected 3")
	}
}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode == 0 {
		t.Fatal("expected test failure, got exit 0")
	}
}
