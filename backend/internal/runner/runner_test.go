package runner

import (
	"strings"
	"testing"
	"time"
)

func TestRun_Go_Playground(t *testing.T) {
	r := New()
	res, err := r.Run(RunRequest{
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
	res, err := r.Run(RunRequest{
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
	res, err := r.Run(RunRequest{
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

func TestRun_UnsupportedLanguage(t *testing.T) {
	r := New()
	_, err := r.Run(RunRequest{Language: "brainfuck", Code: "+++"})
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRun_Go_TestMode_Pass(t *testing.T) {
	r := New()
	res, err := r.Run(RunRequest{
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
	res, err := r.Run(RunRequest{
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
