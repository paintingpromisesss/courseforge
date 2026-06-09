package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LangDriver defines how to run and test code for a specific language.
// Placeholders in command slices: {file}, {testfile}, {dir}.
type LangDriver struct {
	RunCmd    []string          `json:"run_cmd"`    // e.g. ["go", "run", "{file}"]
	TestCmd   []string          `json:"test_cmd"`   // e.g. ["go", "test", "{dir}"]
	Ext       string            `json:"ext"`        // source file extension, e.g. ".go"
	TestExt   string            `json:"test_ext"`   // test file name suffix, e.g. "_test.go"
	InitFiles map[string]string `json:"init_files"` // files written to temp dir before execution, e.g. {"go.mod": "module main\n\ngo 1.26\n"}
}

const (
	defaultTimeout = 10 * time.Second
)

// RunRequest describes a code execution.
type RunRequest struct {
	Language string
	Code     string
	TestCode string        // non-empty → task mode: run tests against Code
	Timeout  time.Duration // 0 → defaultTimeout
}

// RunResult holds the output of an execution.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	TimedOut bool
}

// Runner executes code for configured languages.
type Runner struct {
	mu      sync.RWMutex
	drivers map[string]LangDriver
	file    string // path to runners.json, empty if not set
}

// New creates a Runner with no pre-loaded drivers.
func New() *Runner {
	return &Runner{
		drivers: map[string]LangDriver{
			"go": defaultGoDriver(),
		},
	}
}

// UseFile loads drivers from path (if it exists) and persists future changes there.
func (r *Runner) UseFile(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.file = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r.saveFile()
	}
	if err != nil {
		return fmt.Errorf("read runners file: %w", err)
	}
	var extras map[string]LangDriver
	if err := json.Unmarshal(data, &extras); err != nil {
		return fmt.Errorf("parse runners file: %w", err)
	}
	maps.Copy(r.drivers, extras)
	return nil
}

// HasDriver reports whether a driver for lang is registered.
func (r *Runner) HasDriver(lang string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.drivers[lang]
	return ok
}

// AddDriver adds or replaces a language driver at runtime.
func (r *Runner) AddDriver(lang string, d LangDriver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[lang] = d
	_ = r.saveFile()
}

// RemoveDriver removes a language driver at runtime.
func (r *Runner) RemoveDriver(lang string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.drivers, lang)
	_ = r.saveFile()
}

// saveFile writes all current drivers to r.file. Must be called with mu held.
func (r *Runner) saveFile() error {
	if r.file == "" {
		return nil
	}
	data, err := json.MarshalIndent(r.drivers, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, r.file)
}

// Drivers returns a snapshot of all registered language drivers.
func (r *Runner) Drivers() map[string]LangDriver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return maps.Clone(r.drivers)
}

// Run executes req and returns the result.
func (r *Runner) Run(req RunRequest) (RunResult, error) {
	r.mu.RLock()
	driver, ok := r.drivers[req.Language]
	r.mu.RUnlock()
	if !ok {
		return RunResult{}, fmt.Errorf("unsupported language: %q", req.Language)
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	dir, err := os.MkdirTemp("", "cf-run-*")
	if err != nil {
		return RunResult{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	for name, content := range driver.InitFiles {
		path := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return RunResult{}, fmt.Errorf("write init file %s: %w", name, err)
		}
	}

	codeFile := filepath.Join(dir, "main"+driver.Ext)
	if err := os.WriteFile(codeFile, []byte(req.Code), 0600); err != nil {
		return RunResult{}, fmt.Errorf("write code: %w", err)
	}

	var cmdTemplate []string
	var testFile string

	if req.TestCode != "" {
		testFile = filepath.Join(dir, "main"+driver.TestExt)
		if err := os.WriteFile(testFile, []byte(req.TestCode), 0600); err != nil {
			return RunResult{}, fmt.Errorf("write test: %w", err)
		}
		cmdTemplate = driver.TestCmd
	} else {
		cmdTemplate = driver.RunCmd
	}

	args := expand(cmdTemplate, codeFile, testFile, dir)

	// Resolve relative executable path against the server's CWD before
	// setting cmd.Dir, because Go resolves relative paths relative to cmd.Dir.
	if !filepath.IsAbs(args[0]) && filepath.Base(args[0]) != args[0] {
		if abs, err := filepath.Abs(args[0]); err == nil {
			args[0] = abs
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	configureCommand(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return RunResult{}, fmt.Errorf("exec: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var (
		runErr   error
		timedOut bool
	)
	select {
	case runErr = <-done:
	case <-time.After(timeout):
		timedOut = true
		_ = stopCommand(cmd)
		if err, exited := waitAfterStop(cmd, done); exited {
			runErr = err
		} else {
			return RunResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Duration: time.Since(start),
				TimedOut: true,
			}, nil
		}
	}

	duration := time.Since(start)

	result := RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		TimedOut: timedOut,
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if !result.TimedOut {
			return result, fmt.Errorf("exec: %w", runErr)
		}
	}

	return result, nil
}

func expand(tmpl []string, file, testFile, dir string) []string {
	out := make([]string, len(tmpl))
	for i, s := range tmpl {
		s = strings.ReplaceAll(s, "{file}", file)
		s = strings.ReplaceAll(s, "{testfile}", testFile)
		s = strings.ReplaceAll(s, "{dir}", dir)
		out[i] = s
	}
	return out
}
