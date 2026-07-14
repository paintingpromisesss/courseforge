package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LangDriver defines how to run and test code for a specific language.
// Placeholders in command slices: {file}, {testfile}, {dir}.
type LangDriver struct {
	RunCmd      []string          `json:"run_cmd"`      // e.g. ["go", "run", "{file}"]
	TestCmd     []string          `json:"test_cmd"`     // e.g. ["go", "test", "{dir}"]
	Ext         string            `json:"ext"`          // source file extension, e.g. ".go"
	TestExt     string            `json:"test_ext"`     // test file name suffix, e.g. "_test.go"
	InitFiles   map[string]string `json:"init_files"`   // files written to temp dir before execution, e.g. {"go.mod": "module main\n\ngo 1.26\n"}
	NeedsSchema bool              `json:"needs_schema,omitempty"` // if true, create an isolated postgres schema for the run
}

const (
	defaultTimeout = 10 * time.Second
)

// RunRequest describes a code execution.
type RunRequest struct {
	Language    string
	Code        string
	TestCode    string        // non-empty → task mode: run tests against Code
	Schema      string        // schema.sql content for postgres driver
	Timeout     time.Duration // 0 → defaultTimeout
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
	pgHost  string // postgres host for the "postgres" driver (Unix socket dir, or a TCP host on Windows); empty if not configured
	pgPort  int    // postgres port; paired with pgHost
}

// New creates a Runner preloaded with the built-in language drivers.
func New() *Runner {
	return &Runner{
		drivers: defaultDrivers(),
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

// ConfigurePostgres enables the "postgres" driver by pointing schema-
// isolated runs at a running Postgres cluster (host is a Unix socket
// directory on Unix, or a TCP host like "127.0.0.1" on Windows).
// Call once at startup after PostgresManager.Start succeeds; leave unset
// to disable the driver (Run then errors instead of connecting nowhere).
func (r *Runner) ConfigurePostgres(host string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pgHost = host
	r.pgPort = port
}

// newSchemaName generates a unique, safe-to-interpolate schema name for
// one run. The fixed "cf_run_" prefix + hex charset lets callers embed it
// directly in SQL without quoting (see createSchema/dropSchema and
// PostgresManager.reapOrphanSchemas, which all rely on this exact shape).
func newSchemaName() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return "cf_run_" + hex.EncodeToString(b[:]), nil
}

// createSchema creates an isolated schema for one run in the shared
// "courseforge" database.
func (r *Runner) createSchema(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "psql", "-h", r.pgHost, "-p", strconv.Itoa(r.pgPort), "-d", "courseforge",
		"-v", "ON_ERROR_STOP=1", "-c", "CREATE SCHEMA "+name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create schema: %w: %s", err, out)
	}
	return nil
}

// dropSchema removes a run's schema. Best-effort: if this fails (e.g. the
// process is being killed), PostgresManager's startup reaper catches it
// next boot — the database never accumulates permanent clutter.
func (r *Runner) dropSchema(name string) {
	_ = exec.Command("psql", "-h", r.pgHost, "-p", strconv.Itoa(r.pgPort), "-d", "courseforge",
		"-c", "DROP SCHEMA IF EXISTS "+name+" CASCADE").Run()
}

// saveFile writes user-owned drivers to r.file. Must be called with mu held.
// Built-in drivers left untouched from their factory defaults are NOT written,
// so shipped driver updates keep taking effect instead of being frozen on disk.
func (r *Runner) saveFile() error {
	if r.file == "" {
		return nil
	}
	defaults := defaultDrivers()
	persist := make(map[string]LangDriver, len(r.drivers))
	for lang, d := range r.drivers {
		if def, ok := defaults[lang]; ok && reflect.DeepEqual(d, def) {
			continue // unchanged built-in — let the code own it
		}
		persist[lang] = d
	}
	data, err := json.MarshalIndent(persist, "", "  ")
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

// DefaultDrivers returns the built-in factory drivers, ignoring any user edits.
// Used to reset an edited runner back to its shipped command set.
func (r *Runner) DefaultDrivers() map[string]LangDriver {
	return defaultDrivers()
}

// Run executes req and returns the result. The process is killed when ctx is
// cancelled (e.g. the client disconnects) or the per-request timeout elapses.
func (r *Runner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
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
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var schemaName string
	if driver.NeedsSchema {
		r.mu.RLock()
		host := r.pgHost
		r.mu.RUnlock()
		if host == "" {
			return RunResult{}, fmt.Errorf("postgres driver needs ConfigurePostgres, but not configured")
		}
		var err error
		schemaName, err = newSchemaName()
		if err != nil {
			return RunResult{}, err
		}
		if err := r.createSchema(ctx, schemaName); err != nil {
			return RunResult{}, err
		}
		defer r.dropSchema(schemaName)
	}

	dir, args, err := r.prepare(driver, req)
	if err != nil {
		return RunResult{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if schemaName != "" {
		cmd.Env = append(os.Environ(),
			"PGHOST="+r.pgHost,
			"PGPORT="+strconv.Itoa(r.pgPort),
			"PGDATABASE=courseforge",
			"PGOPTIONS=--search_path="+schemaName+",public",
		)
	}
	configureCommand(cmd)

	// syncBuffer, not bytes.Buffer: on the timeout/cancel path we may read the
	// captured output while a not-yet-dead process is still being copied into it.
	var stdout, stderr syncBuffer
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
	case <-ctx.Done():
		// deadline → timeout; any other cancellation → client went away.
		timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		_ = stopCommand(cmd)
		if err, exited := waitAfterStop(cmd, done); exited {
			runErr = err
		} else {
			return RunResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Duration: time.Since(start),
				TimedOut: timedOut,
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

// prepare builds an isolated temp workspace for req: it writes the driver's
// init files, the submitted code, and (in task mode) the test file, then
// expands the command line. The caller owns dir and must remove it; on any
// failure prepare cleans up the half-built workspace itself.
func (r *Runner) prepare(driver LangDriver, req RunRequest) (dir string, args []string, err error) {
	dir, err = os.MkdirTemp("", "cf-run-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
			dir = ""
		}
	}()

	for name, content := range driver.InitFiles {
		path := filepath.Join(dir, filepath.Base(name))
		if err = os.WriteFile(path, []byte(content), 0600); err != nil {
			return dir, nil, fmt.Errorf("write init file %s: %w", name, err)
		}
	}

	// Write schema.sql if provided (postgres tasks)
	if req.Schema != "" {
		schemaPath := filepath.Join(dir, "schema.sql")
		if err = os.WriteFile(schemaPath, []byte(req.Schema), 0600); err != nil {
			return dir, nil, fmt.Errorf("write schema: %w", err)
		}
	}

	// Course test files reference the submitted code as "solution" (C++
	// #include, JS/Python import) — Java additionally requires the public
	// class name to match the file name exactly, hence "Solution"/"SolutionTest".
	codeBase, testBase := "solution", "solution"
	if req.Language == "java" {
		codeBase, testBase = "Solution", "SolutionTest"
	}

	code := req.Code
	// Wrap only for test runs: pg_prove's fresh connection needs the query
	// persisted as a view. A playground run should print the rows as-is.
	if req.Language == "postgres" && req.TestCode != "" {
		code = wrapPostgresQueryAsView(code)
	}

	// {file}/{testfile} expand to names relative to the temp dir (the command
	// runs with cmd.Dir = dir): the fixed names never contain spaces, so the
	// `cmd /c "..."`/`sh -c "..."` templates stay intact even when the temp
	// dir path itself has one (e.g. a Windows user name with a space).
	codeName := codeBase + driver.Ext
	if err = os.WriteFile(filepath.Join(dir, codeName), []byte(code), 0600); err != nil {
		return dir, nil, fmt.Errorf("write code: %w", err)
	}

	cmdTemplate := driver.RunCmd
	var testName string
	if req.TestCode != "" {
		if req.Language == "java" {
			testName = testBase + driver.Ext
		} else {
			testName = testBase + driver.TestExt
		}
		if err = os.WriteFile(filepath.Join(dir, testName), []byte(req.TestCode), 0600); err != nil {
			return dir, nil, fmt.Errorf("write test: %w", err)
		}
		cmdTemplate = driver.TestCmd
	}

	args = expand(cmdTemplate, codeName, testName, dir)

	// Resolve relative executable path against the server's CWD before the
	// caller sets cmd.Dir, because Go resolves relative paths against cmd.Dir.
	if !filepath.IsAbs(args[0]) && filepath.Base(args[0]) != args[0] {
		if abs, e := filepath.Abs(args[0]); e == nil {
			args[0] = abs
		}
	}
	return dir, args, nil
}

// syncBuffer is a bytes.Buffer safe for concurrent Write (by os/exec's output
// copier goroutine) and String (by Run when it reads output after a timeout or
// cancellation, before the process has fully exited).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
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
