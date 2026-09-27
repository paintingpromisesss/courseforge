// Package sync implements the cloud-vault sync engine: a mirror git repo at
// {dataDir}/sync/repo is snapshotted from local courses + progress and pushed
// to a personal remote. Pull copies the snapshot back. Merges never happen —
// last push wins, history is kept for rollback.
package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

const pushAttempts = 3

// debouncer coalesces rapid notifications into one delayed call.
type debouncer struct {
	mu   stdsync.Mutex
	d    time.Duration
	fn   func()
	timer *time.Timer
}

func newDebouncer(d time.Duration, fn func()) *debouncer {
	return &debouncer{d: d, fn: fn}
}

// notify (re)arms the timer; fn runs once after d of silence.
func (d *debouncer) notify() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Reset(d.d)
		return
	}
	d.timer = time.AfterFunc(d.d, func() {
		d.mu.Lock()
		d.timer = nil
		d.mu.Unlock()
		d.fn()
	})
}

// stop cancels a pending call (shutdown).
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// Engine serializes all sync operations. busy is exposed through Status so
// the UI can poll while a push/pull runs.
type Engine struct {
	mu         stdsync.Mutex
	git        *git.Service
	cfgRepo    *repo.SyncConfigRepository
	sources    *repo.CourseSourcesRepository
	coursesDir string
	dataDir    string
	mirrorDir  string // {dataDir}/sync/repo
	busy       atomic.Bool
	// OnReload is called after Pull changed on-disk courses; DI wires it to
	// the handler-side course re-parse.
	OnReload func()
	// progressDeb debounces on-progress pushes (SetTriggers/Start arm it).
	progressDeb *debouncer
	ticker      *time.Ticker
}

// Status is the sync state for API responses.
type Status struct {
	Syncing        bool     `json:"syncing"`
	LastSync       string   `json:"last_sync"`
	Branch         string   `json:"branch"`
	HeadCommit     string   `json:"head_commit"`
	PendingImports []string `json:"pending_imports"` // cloud sources missing locally
}

func NewEngine(g *git.Service, cfgRepo *repo.SyncConfigRepository, sources *repo.CourseSourcesRepository, coursesDir, dataDir string) *Engine {
	return &Engine{
		git:        g,
		cfgRepo:    cfgRepo,
		sources:    sources,
		coursesDir: coursesDir,
		dataDir:    dataDir,
		mirrorDir:  filepath.Join(dataDir, "sync", "repo"),
	}
}

// Configured reports whether an enabled sync with a remote URL is set up.
func (e *Engine) Configured(ctx context.Context) (bool, error) {
	cfg, err := e.cfgRepo.Load(ctx)
	if err != nil {
		return false, err
	}
	return cfg != nil && cfg.Enabled && cfg.RemoteURL != "", nil
}

// loadConfig returns the enabled config or an error when sync is not set up.
func (e *Engine) loadConfig(ctx context.Context) (*repo.SyncConfig, error) {
	cfg, err := e.cfgRepo.Load(ctx)
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.Enabled || cfg.RemoteURL == "" {
		return nil, errors.New("sync not configured")
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	return cfg, nil
}

// LoadConfig is the exported read used by handlers (nil = not configured).
func (e *Engine) LoadConfig(ctx context.Context) (*repo.SyncConfig, error) {
	return e.cfgRepo.Load(ctx)
}

// SaveConfig is the exported write used by handlers.
func (e *Engine) SaveConfig(ctx context.Context, cfg *repo.SyncConfig) error {
	return e.cfgRepo.Save(ctx, cfg)
}

// ResetMirror wipes the mirror repo (config change: new remote or branch).
func (e *Engine) ResetMirror() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return os.RemoveAll(e.mirrorDir)
}

// Busy reports whether a push/pull/rollback is in flight.
func (e *Engine) Busy() bool { return e.busy.Load() }

// SetTriggers configures automatic push behavior. Called by DI at startup and
// whenever the config is patched.
func (e *Engine) SetTriggers(cfg *repo.SyncConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cfg == nil || !cfg.Enabled || !cfg.Triggers.OnProgress {
		if e.progressDeb != nil {
			e.progressDeb.stop()
			e.progressDeb = nil
		}
		return
	}
	if e.progressDeb == nil {
		e.progressDeb = newDebouncer(30*time.Second, func() {
			if err := e.Push(context.Background()); err != nil {
				stdlog.Printf("sync: on-progress push: %v", err)
			}
		})
	}
}

// NotifyProgress fires the debounced on-progress push trigger.
func (e *Engine) NotifyProgress() {
	e.mu.Lock()
	d := e.progressDeb
	e.mu.Unlock()
	if d != nil {
		d.notify()
	}
}

// Start runs the interval ticker until ctx is cancelled. IntervalMin <= 0
// disables periodic pushes (Stop also works standalone).
func (e *Engine) Start(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ticker != nil {
		return
	}
	cfg, err := e.cfgRepo.Load(ctx)
	if err != nil || cfg == nil || !cfg.Enabled || cfg.Triggers.IntervalMin <= 0 {
		return
	}
	interval := time.Duration(cfg.Triggers.IntervalMin) * time.Minute
	e.ticker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.ticker.C:
				if err := e.Push(context.Background()); err != nil {
					stdlog.Printf("sync: interval push: %v", err)
				}
			}
		}
	}()
}

// Stop cancels the interval ticker and any pending debounced push.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ticker != nil {
		e.ticker.Stop()
		e.ticker = nil
	}
	if e.progressDeb != nil {
		e.progressDeb.stop()
		e.progressDeb = nil
	}
}

// History returns up to limit sync commits (nil when no mirror yet).
func (e *Engine) History(ctx context.Context, limit int) ([]git.Commit, error) {
	if _, err := os.Stat(filepath.Join(e.mirrorDir, ".git")); err != nil {
		return nil, nil
	}
	return e.git.Log(ctx, e.mirrorDir, limit)
}

// CommitFiles lists files touched by a mirror commit.
func (e *Engine) CommitFiles(ctx context.Context, hash string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(e.mirrorDir, ".git")); err != nil {
		return nil, nil
	}
	return e.git.CommitFiles(ctx, e.mirrorDir, hash)
}

// ensureMirror creates {dataDir}/sync/repo on first use.
func (e *Engine) ensureMirror(ctx context.Context, cfg *repo.SyncConfig) error {
	if _, err := os.Stat(filepath.Join(e.mirrorDir, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(e.mirrorDir, 0755); err != nil {
		return err
	}
	if err := e.git.Init(ctx, e.mirrorDir, cfg.RemoteURL, cfg.Branch); err != nil {
		return err
	}
	// Snapshot content must round-trip byte-exact: a global core.autocrlf
	// (typical on Windows) would rewrite every file on checkout.
	return e.git.SetLocalConfig(ctx, e.mirrorDir, "core.autocrlf", "false")
}

// Push snapshots local state into the mirror and pushes it. On rejection it
// re-aligns to origin and retries (never force-pushes).
func (e *Engine) Push(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.busy.Store(true)
	defer e.busy.Store(false)

	cfg, err := e.loadConfig(ctx)
	if err != nil {
		return err
	}
	if err := e.ensureMirror(ctx, cfg); err != nil {
		return fmt.Errorf("init sync mirror: %w", err)
	}

	hostname, _ := os.Hostname()
	var lastErr error
	for attempt := 0; attempt < pushAttempts; attempt++ {
		if err := e.snapshot(ctx, cfg); err != nil {
			return err
		}
		msg := fmt.Sprintf("sync: %s %s", hostname, time.Now().UTC().Format(time.RFC3339))
		if err := e.git.CommitAll(ctx, e.mirrorDir, msg); err != nil {
			return fmt.Errorf("commit snapshot: %w", err)
		}
		if err := e.git.Push(ctx, e.mirrorDir, cfg.Branch); err != nil {
			lastErr = err // remote moved — realign and retry
			continue
		}
		cfg.LastSync = time.Now().UTC().Format(time.RFC3339)
		return e.cfgRepo.Save(ctx, cfg)
	}
	return fmt.Errorf("remote changed, retry later: %w", lastErr)
}

// snapshot aligns the mirror to origin and copies the local state into it.
func (e *Engine) snapshot(ctx context.Context, cfg *repo.SyncConfig) error {
	// A brand-new empty remote has nothing to fetch; tolerate failure here —
	// RemoteBranches below decides whether an align is possible at all.
	_ = e.git.Fetch(ctx, e.mirrorDir)
	branches, err := e.git.RemoteBranches(ctx, e.mirrorDir)
	if err == nil && containsStr(branches, cfg.Branch) {
		if err := e.git.CheckoutBranch(ctx, e.mirrorDir, cfg.Branch, true); err != nil {
			return fmt.Errorf("align mirror to origin: %w", err)
		}
	}
	if err := wipeExceptGit(e.mirrorDir); err != nil {
		return err
	}

	sources, err := e.sources.All(ctx)
	if err != nil {
		return err
	}
	excluded := make(map[string]bool, len(cfg.Exclude))
	for _, x := range cfg.Exclude {
		excluded[x] = true
	}

	entries, err := os.ReadDir(e.coursesDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, en := range entries {
		if !en.IsDir() {
			continue
		}
		name := en.Name()
		// Git-imported courses live in their own remote; excluded stay local.
		if excluded[name] || isImported(sources, name) {
			continue
		}
		if err := copyTree(filepath.Join(e.coursesDir, name), filepath.Join(e.mirrorDir, "courses", name)); err != nil {
			return fmt.Errorf("copy course %s: %w", name, err)
		}
	}

	progressDir := filepath.Join(e.dataDir, "progress")
	if _, err := os.Stat(progressDir); err == nil {
		if err := copyTree(progressDir, filepath.Join(e.mirrorDir, "progress")); err != nil {
			return fmt.Errorf("copy progress: %w", err)
		}
	}

	data, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.mirrorDir, "sources.json"), data, 0644)
}

// Pull aligns the mirror to the cloud snapshot and copies it back. It never
// deletes local courses: git-imports and excluded dirs are skipped, courses
// absent from the cloud stay on disk (the next Push restores them).
func (e *Engine) Pull(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.busy.Store(true)
	defer e.busy.Store(false)

	cfg, err := e.loadConfig(ctx)
	if err != nil {
		return err
	}
	if err := e.ensureMirror(ctx, cfg); err != nil {
		return fmt.Errorf("init sync mirror: %w", err)
	}
	_ = e.git.Fetch(ctx, e.mirrorDir)
	branches, err := e.git.RemoteBranches(ctx, e.mirrorDir)
	if err != nil || !containsStr(branches, cfg.Branch) {
		return errors.New("cloud vault is empty — push first")
	}
	if err := e.git.CheckoutBranch(ctx, e.mirrorDir, cfg.Branch, true); err != nil {
		return fmt.Errorf("align mirror to origin: %w", err)
	}
	if err := e.applyMirror(ctx, cfg); err != nil {
		return err
	}
	if e.OnReload != nil {
		e.OnReload()
	}
	cfg.LastSync = time.Now().UTC().Format(time.RFC3339)
	return e.cfgRepo.Save(ctx, cfg)
}

// applyMirror copies the current mirror working tree back into coursesDir and
// progressDir, then merges cloud sources. Pull and Rollback share it.
func (e *Engine) applyMirror(ctx context.Context, cfg *repo.SyncConfig) error {

	sources, err := e.sources.All(ctx)
	if err != nil {
		return err
	}
	excluded := make(map[string]bool, len(cfg.Exclude))
	for _, x := range cfg.Exclude {
		excluded[x] = true
	}

	mirrorCourses := filepath.Join(e.mirrorDir, "courses")
	entries, err := os.ReadDir(mirrorCourses)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, en := range entries {
		if !en.IsDir() {
			continue
		}
		name := en.Name()
		if excluded[name] || isImported(sources, name) {
			continue
		}
		if err := replaceDir(filepath.Join(mirrorCourses, name), filepath.Join(e.coursesDir, name)); err != nil {
			return fmt.Errorf("restore course %s: %w", name, err)
		}
	}

	if _, err := os.Stat(filepath.Join(e.mirrorDir, "progress")); err == nil {
		if err := replaceDir(filepath.Join(e.mirrorDir, "progress"), filepath.Join(e.dataDir, "progress")); err != nil {
			return fmt.Errorf("restore progress: %w", err)
		}
	}

	if err := e.mergeSources(ctx); err != nil {
		return err
	}
	return nil
}

// mergeSources records cloud source entries that are missing locally.
func (e *Engine) mergeSources(ctx context.Context) error {
	data, err := os.ReadFile(filepath.Join(e.mirrorDir, "sources.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var cloud map[string]repo.CourseSource
	if err := json.Unmarshal(data, &cloud); err != nil {
		return err
	}
	local, err := e.sources.All(ctx)
	if err != nil {
		return err
	}
	for dir, src := range cloud {
		if _, ok := local[dir]; !ok {
			if err := e.sources.Set(ctx, dir, src); err != nil {
				return err
			}
		}
	}
	return nil
}

// Status reports the current sync state without touching the network.
func (e *Engine) Status(ctx context.Context) (*Status, error) {
	st := &Status{Syncing: e.busy.Load()}
	cfg, err := e.cfgRepo.Load(ctx)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		st.LastSync = cfg.LastSync
		st.Branch = cfg.Branch
	}
	if _, err := os.Stat(filepath.Join(e.mirrorDir, ".git")); err == nil {
		if head, err := e.git.HeadCommit(ctx, e.mirrorDir); err == nil {
			st.HeadCommit = head
		}
	}
	sources, err := e.sources.All(ctx)
	if err != nil {
		return nil, err
	}
	for dir := range sources {
		if _, err := os.Stat(filepath.Join(e.coursesDir, filepath.FromSlash(dir))); errors.Is(err, os.ErrNotExist) {
			st.PendingImports = append(st.PendingImports, dir)
		}
	}
	return st, nil
}

// Rollback restores the local state to the given mirror commit: the commit's
// tree is checked out in the mirror, copied back like Pull, committed and
// pushed — history keeps the intermediate commits.
func (e *Engine) Rollback(ctx context.Context, commit string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.busy.Store(true)
	defer e.busy.Store(false)

	cfg, err := e.loadConfig(ctx)
	if err != nil {
		return err
	}
	if err := e.ensureMirror(ctx, cfg); err != nil {
		return fmt.Errorf("init sync mirror: %w", err)
	}
	_ = e.git.Fetch(ctx, e.mirrorDir)
	if err := e.git.CheckoutPaths(ctx, e.mirrorDir, commit); err != nil {
		return fmt.Errorf("unknown sync commit: %w", err)
	}
	if err := e.applyMirror(ctx, cfg); err != nil {
		return err
	}
	if err := e.git.CommitAll(ctx, e.mirrorDir, fmt.Sprintf("rollback to %s", shortCommit(commit))); err != nil {
		return fmt.Errorf("commit rollback: %w", err)
	}
	hostname, _ := os.Hostname()
	var lastErr error
	for attempt := 0; attempt < pushAttempts; attempt++ {
		if err := e.git.Push(ctx, e.mirrorDir, cfg.Branch); err != nil {
			lastErr = err
			if berr := e.git.CheckoutBranch(ctx, e.mirrorDir, cfg.Branch, true); berr == nil {
				// realigned after a race; replay the rollback content on top
				if cerr := e.git.CheckoutPaths(ctx, e.mirrorDir, commit); cerr != nil {
					return fmt.Errorf("replay rollback: %w", cerr)
				}
				if cerr := e.git.CommitAll(ctx, e.mirrorDir, fmt.Sprintf("rollback to %s (%s)", shortCommit(commit), hostname)); cerr != nil {
					return fmt.Errorf("commit rollback replay: %w", cerr)
				}
			}
			continue
		}
		cfg.LastSync = time.Now().UTC().Format(time.RFC3339)
		return e.cfgRepo.Save(ctx, cfg)
	}
	return fmt.Errorf("remote changed, retry later: %w", lastErr)
}

func shortCommit(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// isImported reports whether a top-level courses dir is git-imported. Source
// keys are courseDir paths; a catalog import owns everything nested under it.
func isImported(sources map[string]repo.CourseSource, name string) bool {
	for dir := range sources {
		d := filepath.ToSlash(dir)
		if d == name || strings.HasPrefix(d, name+"/") {
			return true
		}
	}
	return false
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// wipeExceptGit removes every top-level entry of dir except .git.
func wipeExceptGit(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, en := range entries {
		if en.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, en.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies src into dst, skipping any nested .git directory.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// replaceDir swaps dst for a copy of src: copy to <dst>.synctmp, drop the old
// dir, rename. The rename window is tiny and the tmp copy is already verified.
func replaceDir(src, dst string) error {
	tmp := dst + ".synctmp"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := copyTree(src, tmp); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	return nil
}
