# GitHub Import + Cloud Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import courses from GitHub repos (branch switching, PAT auth) and sync all courses + progress to a personal GitHub "cloud vault" with history and rollback.

**Architecture:** System git CLI wrapped in `internal/infrastructure/git` (token via `GIT_CONFIG_*` env, never in argv or `.git/config`). Phase 1: `POST /api/git/import` clones into temp, validates via existing course parser, moves into `courses/` with `.git` intact; per-course branch/checkout/pull endpoints; source metadata in `data-dir/course_sources.json`. Phase 2: mirror repo in `data-dir/sync/repo` — snapshot-on-remote (fetch → align to origin → copy local state → commit → push, retry ×3), no merges ever; pull = copy-back; rollback = copy-back from commit + snapshot-commit. Progress moves from course dir to `data-dir/progress/` first.

**Tech Stack:** Go 1.26 (chi, existing repos/services patterns), git CLI ≥2.x, React 19 + TanStack Query + existing SettingsPanel lazy-section pattern.

**Spec:** `docs/superpowers/specs/2026-09-27-github-import-cloud-sync-design.md`

## Global Constraints

- Branch `feat/github-sync`. Local commits freely; **push only after user approval**.
- Module path `github.com/paintingpromisesss/courseforge`.
- Backend tests: `cd backend && make test` (Makefile sets GOCACHE). Single package: `cd backend && go test ./internal/infrastructure/git/...` etc.
- Frontend tests: `cd frontend && npx vitest run <file>`. TS strict incl. `noUnusedLocals`, `verbatimModuleSyntax`.
- API convention: camelCase query params, snake_case JSON bodies (`api/client.ts` quirk — intentional).
- JSON config repos follow `repo/ai_config.go`: file in dataDir, atomic write via `.tmp` + rename, `Load` returns `(nil, nil)` on NotExist.
- Handlers hold `h.mu` (RWMutex) when touching `h.courses`/`h.catalogs`; file ops outside the lock (see `deleteCourse` in `import.go`).
- Registration after disk changes: `h.loadAndRegisterCourse(slug)` / `h.loadAndRegisterCatalog(dirSlug)` (both in `import.go`).
- All new routes get swagger annotations; run `cd backend && make swagger` before final build.
- Never return the raw PAT in any response; mask as first 4 + last 4 chars.
- Russian UI copy (all existing UI is Russian).

## Review Focus

1. **Token leakage** — PAT must not appear in process args, `.git/config`, logs, or API responses. Pinned by Task 2 test (env-only injection) and Task 3 test (masking).
2. **Dirty working tree on checkout/pull** — a user who edited course files must not lose them silently; checkout refuses on dirty tree unless `force`. Pinned by Task 6.
3. **Progress loss during migration** — old `courses/<dir>/progress.json` must move, not vanish; migration must be idempotent. Pinned by Task 1.
4. **Sync race (two devices)** — push after concurrent remote update must retry align→snapshot→push, not force-push. Pinned by Task 9.
5. **Invalid course after branch switch** — checkout that yields unparseable YAML must roll back to previous commit and return 422. Pinned by Task 6.

---

### Task 1: Progress repository moves to dataDir

**Files:**
- Modify: `backend/internal/infrastructure/repo/progress.go` (constructor + `progressPath`)
- Modify: `backend/internal/infrastructure/repo/progress_test.go`
- Modify: `backend/internal/di/di.go:71` (`repo.NewFileProgressRepository(cfg.CoursesDir)` → new signature)
- Modify: `backend/internal/mcp/*.go` if it constructs the progress repo (grep `NewFileProgressRepository` — di.go:92 passes `pr` into `mcp.NewCourseForgeProvider`, so only di.go constructs it)

**Interfaces:**
- Produces: `func NewFileProgressRepository(dataDir string) *FileProgressRepository` — stores files at `{dataDir}/progress/{courseDir}/progress.json`; `func MigrateProgress(coursesDir, dataDir string) error` — moves legacy files (idempotent).

- [ ] **Step 1: Write failing tests** — in `progress_test.go`:

```go
func TestProgressStoredInDataDir(t *testing.T) {
	dataDir := t.TempDir()
	r := NewFileProgressRepository(dataDir)
	if err := r.MarkDone(context.Background(), "go-basics", "go-basics", "task-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "progress", "go-basics", "progress.json")); err != nil {
		t.Fatalf("progress file not in dataDir: %v", err)
	}
}

func TestMigrateProgress(t *testing.T) {
	coursesDir, dataDir := t.TempDir(), t.TempDir()
	legacy := filepath.Join(coursesDir, "cat", "course1")
	os.MkdirAll(legacy, 0755)
	os.WriteFile(filepath.Join(legacy, "progress.json"), []byte(`{"course_slug":"course1","completed_tasks":{"a":true}}`), 0644)
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "progress", "cat", "course1", "progress.json")); err != nil {
		t.Fatalf("not migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "progress.json")); !os.IsNotExist(err) {
		t.Fatal("legacy file not removed")
	}
	// idempotent: second run must not error and must not resurrect
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	// never overwrite an existing new-location file
	os.WriteFile(filepath.Join(legacy, "progress.json"), []byte(`{}`), 0644)
	if err := MigrateProgress(coursesDir, dataDir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dataDir, "progress", "cat", "course1", "progress.json"))
	if !strings.Contains(string(data), "course1") {
		t.Fatal("migration overwrote existing progress")
	}
}
```

- [ ] **Step 2: Run, verify FAIL** — `cd backend && go test ./internal/infrastructure/repo/ -run "TestProgressStoredInDataDir|TestMigrateProgress"`

- [ ] **Step 3: Implement** — in `progress.go`:

```go
// FileProgressRepository reads and writes per-course progress.json files.
// Files live at {dataDir}/progress/{courseDir}/progress.json.
type FileProgressRepository struct {
	mu         sync.Mutex
	progressDir string
}

func NewFileProgressRepository(dataDir string) *FileProgressRepository {
	return &FileProgressRepository{progressDir: filepath.Join(dataDir, "progress")}
}

func (s *FileProgressRepository) progressPath(courseDir string) string {
	return filepath.Join(s.progressDir, filepath.FromSlash(courseDir), "progress.json")
}
```

`save` must `os.MkdirAll(filepath.Dir(path), 0755)` before writing. Add:

```go
// MigrateProgress moves legacy {coursesDir}/**/progress.json files into
// {dataDir}/progress/, preserving relative paths. Idempotent; never
// overwrites a file that already exists at the destination.
func MigrateProgress(coursesDir, dataDir string) error {
	dest := filepath.Join(dataDir, "progress")
	return filepath.WalkDir(coursesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable course dirs are not our problem
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if d.IsDir() || d.Name() != "progress.json" {
			return nil
		}
		rel, _ := filepath.Rel(coursesDir, path)
		target := filepath.Join(dest, filepath.Dir(rel), "progress.json")
		if _, err := os.Stat(target); err == nil {
			return os.Remove(path) // destination wins; drop stale legacy copy
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.Rename(path, target); err != nil {
			// cross-device fallback
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if werr := os.WriteFile(target, data, 0644); werr != nil {
				return werr
			}
			return os.Remove(path)
		}
		return nil
	})
}
```

In `di.go` after the MkdirAll loop (line ~36):

```go
if err := repo.MigrateProgress(cfg.CoursesDir, cfg.DataDir); err != nil {
	log.Printf("warning: progress migration: %v", err)
}
```

and change line 71 to `pr := repo.NewFileProgressRepository(cfg.DataDir)`.

- [ ] **Step 4: Run tests** — `cd backend && go test ./internal/infrastructure/repo/ ./internal/application/... ./internal/mcp/...` — all PASS. Fix any constructor-arg compile errors surfaced by grep `NewFileProgressRepository`.

- [ ] **Step 5: Commit**

```bash
git add -A backend
git commit -m "refactor(progress): store progress.json under dataDir, migrate legacy files"
```

---

### Task 2: git wrapper package

**Files:**
- Create: `backend/internal/infrastructure/git/git.go`
- Test: `backend/internal/infrastructure/git/git_test.go`

**Interfaces:**
- Produces (package `git`):
```go
type Service struct{ mu sync.Mutex; token string; timeout, cloneTimeout time.Duration }
func NewService() *Service                       // timeout 60s, cloneTimeout 5m
func (s *Service) SetToken(token string)
func (s *Service) Token() string
func (s *Service) Available() bool               // cached `git --version` probe
type Error struct{ Stderr string; ExitCode int } // error interface; message = trimmed stderr
func (s *Service) Clone(ctx context.Context, url, dir, branch string) error   // branch may be ""
func (s *Service) Fetch(ctx context.Context, dir string) error
func (s *Service) RemoteBranches(ctx context.Context, dir string) ([]string, error) // "git branch -r" minus HEAD, "origin/x" → "x"
func (s *Service) CurrentBranch(ctx context.Context, dir string) (string, error)    // "git rev-parse --abbrev-ref HEAD"
func (s *Service) HeadCommit(ctx context.Context, dir string) (string, error)       // "git rev-parse HEAD"
func (s *Service) CheckoutBranch(ctx context.Context, dir, branch string, force bool) error // fetch + checkout -B branch origin/branch (force: add "clean -fd" + reset --hard first; refuse dirty tree without force)
func (s *Service) PullFF(ctx context.Context, dir string) error                     // "git merge --ff-only origin/<branch>" after fetch
func (s *Service) Dirty(ctx context.Context, dir string) (bool, error)              // "git status --porcelain" non-empty
func (s *Service) Init(ctx context.Context, dir, remoteURL, branch string) error    // init -b branch, remote add origin
func (s *Service) CommitAll(ctx context.Context, dir, message string) error         // add -A; commit --allow-empty-message -m (skip if nothing staged)
func (s *Service) Push(ctx context.Context, dir, branch string) error               // "git push origin branch"; Error on rejection
func (s *Service) Log(ctx context.Context, dir string, limit int) ([]Commit, error) // --format=%H%x1f%at%x1f%an%x1f%s
func (s *Service) CommitFiles(ctx context.Context, dir, hash string) ([]string, error) // "git show --name-only --format=" hash
func (s *Service) CheckoutPaths(ctx context.Context, dir, hash string) error        // "git checkout <hash> -- ." for rollback copy source
type Commit struct{ Hash string; Time time.Time; Author, Subject string }
```

- [ ] **Step 1: Write failing tests** — real git in `t.TempDir()`, skip if unavailable:

```go
func gitAvailable(t *testing.T) {
	t.Helper()
	if exec.Command("git", "--version").Run() != nil {
		t.Skip("git not installed")
	}
}

func makeRemote(t *testing.T) (remoteDir string) {
	t.Helper()
	remoteDir = filepath.Join(t.TempDir(), "remote")
	os.MkdirAll(remoteDir, 0755)
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(remoteDir, "init", "-b", "main")
	os.WriteFile(filepath.Join(remoteDir, "a.txt"), []byte("one"), 0644)
	run(remoteDir, "add", "-A")
	run(remoteDir, "commit", "-m", "init")
	run(remoteDir, "checkout", "-b", "dev")
	os.WriteFile(filepath.Join(remoteDir, "a.txt"), []byte("dev"), 0644)
	run(remoteDir, "commit", "-am", "dev change")
	run(remoteDir, "checkout", "main")
	return remoteDir
}

func TestCloneBranchesCheckoutPull(t *testing.T) {
	gitAvailable(t)
	remote := makeRemote(t)
	s := NewService()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "clone")
	if err := s.Clone(ctx, remote, dir, ""); err != nil {
		t.Fatal(err)
	}
	br, _ := s.CurrentBranch(ctx, dir)
	if br != "main" {
		t.Fatalf("branch = %s", br)
	}
	branches, err := s.RemoteBranches(ctx, dir)
	if err != nil || len(branches) != 2 {
		t.Fatalf("branches = %v, err %v", branches, err)
	}
	if err := s.CheckoutBranch(ctx, dir, "dev", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(data) != "dev" {
		t.Fatalf("checkout did not switch content: %s", data)
	}
	dirty, _ := s.Dirty(ctx, dir)
	if dirty {
		t.Fatal("fresh checkout is dirty")
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local edit"), 0644)
	if err := s.CheckoutBranch(ctx, dir, "main", false); err == nil {
		t.Fatal("checkout must refuse on dirty tree without force")
	}
	if err := s.CheckoutBranch(ctx, dir, "main", true); err != nil {
		t.Fatalf("force checkout: %v", err)
	}
	// ff-only pull
	os.WriteFile(filepath.Join(remote, "b.txt"), []byte("new"), 0644)
	if err := s.CommitAll(ctx, remote, "remote change"); err != nil {
		t.Fatal(err)
	}
	if err := s.Push(ctx, remote, "main"); err != nil { // push from remote dir to itself = no-op; instead commit directly above
		t.Log("note: remote is not bare; pull test uses direct commit")
	}
	if err := s.PullFF(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("pull did not fetch new file: %v", err)
	}
	log, _ := s.Log(ctx, dir, 10)
	if len(log) < 2 || log[0].Subject == "" {
		t.Fatalf("log wrong: %+v", log)
	}
}
```

Note for implementer: `makeRemote` uses a non-bare repo as "remote" — git allows `clone <path>` from it and `fetch` from it, which is all the tests need. Do not push *to* a non-bare checked-out branch; the "remote change" is committed directly in `remote` on `main` and the clone sees it via fetch. Drop the `s.Push(ctx, remote, ...)` block above (it's a no-op leftover) — PullFF fetches from `remote` fine.

Token test:

```go
func TestTokenPassedViaEnvOnly(t *testing.T) {
	gitAvailable(t)
	s := NewService()
	s.SetToken("ghp_secret123")
	// run() must set GIT_CONFIG_COUNT/KEY_0/VALUE_0 — verified by cloning a
	// local repo and checking .git/config contains no token afterwards.
	remote := makeRemote(t)
	dir := filepath.Join(t.TempDir(), "c")
	if err := s.Clone(context.Background(), remote, dir, ""); err != nil {
		t.Fatal(err)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, ".git", "config"))
	if strings.Contains(string(cfg), "secret") {
		t.Fatal("token leaked into .git/config")
	}
}

func TestAvailable(t *testing.T) {
	gitAvailable(t)
	if !NewService().Available() {
		t.Fatal("Available() = false with git installed")
	}
}
```

- [ ] **Step 2: Run, verify FAIL** — `cd backend && go test ./internal/infrastructure/git/` (package doesn't exist).

- [ ] **Step 3: Implement `git.go`** — core runner:

```go
func (s *Service) run(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = dir
	env := os.Environ()
	env = append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=CourseForge", "GIT_AUTHOR_EMAIL=courseforge@local",
		"GIT_COMMITTER_NAME=CourseForge", "GIT_COMMITTER_EMAIL=courseforge@local")
	if s.token != "" {
		env = append(env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.https://github.com/.extraHeader",
			"GIT_CONFIG_VALUE_0=Authorization: Bearer "+s.token)
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		code := -1
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), &Error{Stderr: msg, ExitCode: code}
	}
	return stdout.String(), nil
}
```

All methods are thin wrappers over `run` with the flags listed in Interfaces. `CheckoutBranch` sequence: `fetch origin` → if `!force` and `Dirty` → return Error "working tree has uncommitted changes; commit or discard them first" → if `force`: `clean -fd` then `reset --hard` → `checkout -B <branch> origin/<branch>`. `PullFF`: `fetch origin` → `merge --ff-only origin/<currentBranch>`. `Push`: `push origin <branch>` (no `--force`, ever). `Available`: once-only `exec.Command("git","--version")` with result cached in a field guarded by `sync.Once`. `RemoteBranches`: parse `branch -r` lines, strip `origin/` prefix, skip lines containing `->`. `Log`: parse `%x1f`-separated fields, unix seconds → `time.Unix`.

- [ ] **Step 4: Run tests** — PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/infrastructure/git
git commit -m "feat(git): git CLI wrapper with env-injected token auth"
```

---

### Task 3: git auth storage + endpoints

**Files:**
- Create: `backend/internal/infrastructure/repo/git_auth.go` (+ `git_auth_test.go`)
- Create: `backend/internal/api/handlers/git_auth.go`
- Modify: `backend/internal/api/handlers/handler.go` (add `gitAuth *repo.GitAuthRepository` + `gitSvc *git.Service` fields, extend `New`)
- Modify: `backend/internal/api/handlers/routes.go`
- Modify: `backend/internal/di/di.go` (construct repo + service, wire token, pass into `handlers.New`)

**Interfaces:**
- Produces:
```go
// repo
type GitAuth struct{ Token, Username string } // JSON: {"token": "...", "username": ""}
func NewGitAuthRepository(dataDir string) *GitAuthRepository // file git_auth.json
func (r *GitAuthRepository) Load(ctx context.Context) (*GitAuth, error) // (nil,nil) when absent
func (r *GitAuthRepository) Save(ctx context.Context, a *GitAuth) error
func (r *GitAuthRepository) Delete() error
func MaskToken(t string) string // "ghp_secret123" → "ghp_…t123"; len<9 → "****"
```
- Handler endpoints: `GET /api/git/auth` → `{"configured":bool,"username":string,"token_masked":string}`; `PATCH /api/git/auth` body `{"token":"...","username":"..."}` (empty token = delete); side effect: `h.gitSvc.SetToken(...)`. `POST /api/git/auth/test` → runs `git ls-remote https://github.com/octocat/Hello-World` (public repo) with token env to validate connectivity... **simpler:** call GitHub API `GET https://api.github.com/user` with `Authorization: Bearer` when token set; 200 → `{"ok":true,"login":"..."}`; 401 → `{"ok":false,"error":"invalid token"}`.

- [ ] **Step 1: Failing tests** — `git_auth_test.go`:

```go
func TestGitAuthRoundTrip(t *testing.T) {
	r := NewGitAuthRepository(t.TempDir())
	a, err := r.Load(context.Background())
	if err != nil || a != nil {
		t.Fatalf("empty load = %v, %v", a, err)
	}
	if err := r.Save(context.Background(), &GitAuth{Token: "ghp_abc123def456", Username: "me"}); err != nil {
		t.Fatal(err)
	}
	a, _ = r.Load(context.Background())
	if a.Token != "ghp_abc123def456" || a.Username != "me" {
		t.Fatalf("round trip: %+v", a)
	}
	if err := r.Delete(); err != nil {
		t.Fatal(err)
	}
	a, _ = r.Load(context.Background())
	if a != nil {
		t.Fatal("delete failed")
	}
}

func TestMaskToken(t *testing.T) {
	if m := MaskToken("ghp_abcdefghijklmnop"); m != "ghp_…mnop" {
		t.Fatalf("mask = %s", m)
	}
	if MaskToken("short") != "****" {
		t.Fatal("short mask wrong")
	}
}
```

Handler test in new `git_auth_handler_test.go` (follow `import_test.go` for Handler construction in tests): PATCH with token → GET returns masked, never raw; PATCH with empty token deletes.

- [ ] **Step 2: Verify FAIL** — `cd backend && go test ./internal/infrastructure/repo/ ./internal/api/handlers/`

- [ ] **Step 3: Implement** — repo is a copy of `ai_config.go` shape with `git_auth.json`. Handler file:

```go
// @Summary Get GitHub auth status
// @Tags git
// @Produce json
// @Success 200 {object} dto.GitAuthStatus
// @Router /git/auth [get]
func (h *Handler) getGitAuth(w http.ResponseWriter, r *http.Request) { ... }

// @Summary Set or clear GitHub token
// @Tags git
// @Router /git/auth [patch]
func (h *Handler) patchGitAuth(w http.ResponseWriter, r *http.Request) { ... }

// @Summary Validate token against GitHub API
// @Tags git
// @Router /git/auth/test [post]
func (h *Handler) testGitAuth(w http.ResponseWriter, r *http.Request) {
	// http.Get https://api.github.com/user with Bearer header, 10s timeout;
	// 200 → decode {"login"}; 401/403 → ok:false
}
```

DTOs in `backend/internal/api/dto/git.go`:

```go
type GitAuthStatus struct {
	Configured  bool   `json:"configured"`
	Username    string `json:"username"`
	TokenMasked string `json:"token_masked"`
}
type PatchGitAuthReq struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}
type GitAuthTestResp struct {
	OK    bool   `json:"ok"`
	Login string `json:"login,omitempty"`
	Error string `json:"error,omitempty"`
}
```

Routes (routes.go, after the mcp block):

```go
r.Get("/git/auth", h.getGitAuth)
r.Patch("/git/auth", h.patchGitAuth)
r.Post("/git/auth/test", h.testGitAuth)
```

DI: construct `gitRepo := repo.NewGitAuthRepository(cfg.DataDir)`, `gitSvc := git.NewService()`; load saved auth and `gitSvc.SetToken(a.Token)`; pass both into `handlers.New` (extend signature + Handler struct; update all `handlers.New` call sites — di.go and handler tests).

- [ ] **Step 4: Tests PASS** — repo + handlers packages.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(git): PAT storage and auth endpoints"
```

---

### Task 4: course sources store + GitHub import endpoint

**Files:**
- Create: `backend/internal/infrastructure/repo/course_sources.go` (+ test)
- Create: `backend/internal/api/handlers/git_import.go` (+ test)
- Modify: `backend/internal/api/handlers/handler.go` (add `sources *repo.CourseSourcesRepository`)
- Modify: `backend/internal/api/handlers/routes.go`, `backend/internal/di/di.go`
- Modify: `backend/internal/api/dto/git.go`

**Interfaces:**
- Produces:
```go
// repo
type CourseSource struct {
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	ImportedAt string `json:"imported_at"` // RFC3339, set by caller (time.Now)
}
type CourseSourcesRepository struct{ ... } // file course_sources.json, atomic save
func NewCourseSourcesRepository(dataDir string) *CourseSourcesRepository
func (r *CourseSourcesRepository) All(ctx context.Context) (map[string]CourseSource, error)
func (r *CourseSourcesRepository) Set(ctx context.Context, courseDir string, s CourseSource) error
func (r *CourseSourcesRepository) Delete(ctx context.Context, courseDir string) error
```
```go
// dto
type GitImportReq struct {
	URL    string `json:"url"`
	Branch string `json:"branch"` // optional
}
type GitImportResp struct {
	Slug   string `json:"slug"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}
type GitBranchesResp struct {
	Branches []string `json:"branches"`
	Current  string   `json:"current"`
	Source   *CourseSourceDTO `json:"source"` // repo/branch/commit or null
}
type GitStatusResp struct {
	Branch, Commit string
	Dirty          bool   `json:"dirty"`
}
```
- Handler: `POST /api/git/import` (see flow below).

- [ ] **Step 1: Failing tests** — `course_sources_test.go`: Set → All → Delete round trip; concurrent Set from 2 goroutines (mutex in repo).

`git_import_test.go` — build a **local git repo** containing a minimal valid course (reuse the `validCourse()` fixture shape from `parser_test.go:12`, written to disk instead of MapFS):

```go
func makeCourseRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// write go-interview/course.yaml + track/topic/unit/task files (copy fixture from parser_test.go)
	// git init -b main; add -A; commit
	return dir
}

func TestImportFromGitRepo(t *testing.T) {
	gitAvailable(t)
	repoDir := makeCourseRepo(t)
	h := newTestHandler(t) // same helper as import_test.go
	body := `{"url":` + jsonStr(repoDir) + `}`
	w := postJSON(t, h, "/api/git/import", body)
	if w.Code != 201 { t.Fatalf("status %d: %s", w.Code, w.Body) }
	var resp dto.GitImportResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Slug != "go-interview" { t.Fatal(resp.Slug) }
	if _, err := os.Stat(filepath.Join(h.coursesDir, "go-interview", ".git")); err != nil {
		t.Fatal(".git not preserved")
	}
	if h.getCourseBySlug("go-interview") == nil { t.Fatal("not registered") }
	src, _ := h.sources.All(context.Background())
	if src["go-interview"].Repo != repoDir { t.Fatal("source not recorded") }
}

func TestImportRejectsNonGitHubURLInProductionMode(t *testing.T) {
	// URL validation: only https://github.com/... or (test mode) local paths.
	h := newTestHandler(t)
	w := postJSON(t, h, "/api/git/import", `{"url":"https://gitlab.com/x/y"}`)
	if w.Code != 400 { t.Fatal(w.Code) }
}

func TestImportRejectsRepoWithoutManifest(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0644)
	// git init/add/commit
	w := postJSON(t, h, "/api/git/import", `{"url":`+jsonStr(dir)+`}`)
	if w.Code != 400 { t.Fatal(w.Code) } // "neither course.yaml nor catalog.yaml found"
}
```

For testability, URL validation helper:

```go
// validateRepoURL accepts https://github.com/<owner>/<repo>(.git) and, for
// local testing, existing filesystem paths. Returns normalized URL.
func validateRepoURL(u string) (string, bool)
```

Production check: `https://` scheme + host `github.com` + exactly 2 path segments (+optional `.git`). Local absolute paths pass through (used by tests and harmless — a local clone).

- [ ] **Step 2: Verify FAIL.**

- [ ] **Step 3: Implement `importFromGit` handler flow** (in `git_import.go`):

```go
func (h *Handler) gitImport(w http.ResponseWriter, r *http.Request) {
	if !h.gitSvc.Available() {
		h.writeError(w, http.StatusServiceUnavailable, "git is not installed")
		return
	}
	var req dto.GitImportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	url, ok := validateRepoURL(strings.TrimSpace(req.URL))
	if !ok {
		h.writeError(w, http.StatusBadRequest, "expected a https://github.com/owner/repo URL")
		return
	}
	tmpDir, err := os.MkdirTemp("", "cf-git-import-*")
	if err != nil { h.writeError(w, 500, "temp dir"); return }
	defer os.RemoveAll(tmpDir)
	cloneDir := filepath.Join(tmpDir, "repo")
	if err := h.gitSvc.Clone(r.Context(), url, cloneDir, req.Branch); err != nil {
		var ge *git.Error
		msg := "clone failed: " + err.Error()
		status := http.StatusBadRequest
		if errors.As(err, &ge) && (strings.Contains(ge.Stderr, "Authentication failed") ||
			strings.Contains(ge.Stderr, "could not read Username") ||
			strings.Contains(ge.Stderr, "403")) {
			status = http.StatusUnauthorized
			msg = "repository is private or unavailable — add a GitHub token in Settings"
		}
		h.writeError(w, status, msg)
		return
	}
	// manifest must be at repo root (v1)
	if !fileExists(filepath.Join(cloneDir, "course.yaml")) && !fileExists(filepath.Join(cloneDir, "catalog.yaml")) {
		h.writeError(w, http.StatusBadRequest, "neither course.yaml nor catalog.yaml found in repository root")
		return
	}
	// slug collision check happens inside importFromDir; but the clone dir is
	// named "repo" — importFromDir derives destDir from the parsed slug, and
	// moveDir keeps .git. Rename cloneDir to parsed slug first is unnecessary:
	// importFromDir(sourceDir) parses from sourceDir itself and moves the WHOLE
	// dir (incl. .git) into coursesDir/<slug>.
	slug, herr := h.importFromDir(cloneDir)
	if herr != nil {
		h.writeError(w, herr.status, herr.msg)
		return
	}
	branch, _ := h.gitSvc.CurrentBranch(r.Context(), filepath.Join(h.coursesDir, slug))
	commit, _ := h.gitSvc.HeadCommit(r.Context(), filepath.Join(h.coursesDir, slug))
	_ = h.sources.Set(r.Context(), slug, repo.CourseSource{
		Repo: url, Branch: branch, Commit: commit,
		ImportedAt: time.Now().UTC().Format(time.RFC3339),
	})
	h.writeJSON(w, http.StatusCreated, dto.GitImportResp{Slug: slug, Branch: branch, Commit: commit})
}
```

**Important:** `importFromDir` currently checks `h.getCourseBySlug(c.Slug)` and destDir collision → 409, good. It also does `courseparser.LoadOne(filepath.Dir(sourceDir), filepath.Base(sourceDir))` — since cloneDir basename is "repo" but slug comes from YAML, check `importCourseDir`: it moves `sourceDir` → `coursesDir/<c.Slug>`. Works with any basename. **Catalog path caveat:** `dedupeImportedCatalog` renames nested courses — `.git` stays at catalog root, fine.

Route: `r.Post("/git/import", h.gitImport)`.

`deleteCourse`/`deleteCatalog` must also `h.sources.Delete(ctx, dir)` — add to both (import.go).

- [ ] **Step 4: Tests PASS** (`go test ./internal/infrastructure/repo/ ./internal/api/handlers/`).

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(git): import course/catalog from GitHub URL"
```

---

### Task 5: per-course branches / checkout / pull / status endpoints

**Files:**
- Create: `backend/internal/api/handlers/git_course.go` (+ test)
- Modify: `backend/internal/api/handlers/routes.go`

**Interfaces:**
- Consumes: `git.Service` (Task 2), `h.sources` (Task 4), `h.loadAndRegisterCourse/Catalog` (existing).
- Produces routes:
  - `GET /courses/{courseSlug}/git/branches` → `dto.GitBranchesResp`
  - `POST /courses/{courseSlug}/git/checkout` body `{"branch":"dev","force":false}` → `dto.GitImportResp`
  - `POST /courses/{courseSlug}/git/pull` → `dto.GitImportResp`
  - `GET /courses/{courseSlug}/git/status` → `dto.GitStatusResp`
  - All 404 if no source metadata for the course; 503 if git unavailable; 409 if dirty without force.

- [ ] **Step 1: Failing tests** — extend the Task-4 fixture: import course, then create branch `dev` in the source repo with a changed `title:` in course.yaml; `GET branches` lists both; `POST checkout {branch:"dev"}` changes registered course title; checkout with dirty tree → 409; checkout with `force:true` succeeds; checkout to a branch whose YAML is broken (delete course.yaml on branch `broken`) → 422 **and course still parses** (rolled back to previous commit); `POST pull` after source-repo commit → updated title.

Broken-branch rollback test:

```go
func TestCheckoutInvalidBranchRollsBack(t *testing.T) {
	// branch "broken": course.yaml has invalid slug (≠ folder name)
	w := postJSON(t, h, "/api/courses/go-interview/git/checkout", `{"branch":"broken"}`)
	if w.Code != 422 { t.Fatalf("status %d", w.Code) }
	if h.getCourseBySlug("go-interview") == nil { t.Fatal("course lost after failed checkout") }
	// on-disk content back at previous branch
	branch, _ := h.gitSvc.CurrentBranch(context.Background(), courseDir)
	if branch != "main" { t.Fatalf("branch = %s", branch) }
}
```

- [ ] **Step 2: Verify FAIL.**

- [ ] **Step 3: Implement** — `git_course.go`:

```go
func (h *Handler) gitCourseDir(r *http.Request) (slug, dir string, ok bool) {
	slug = chi.URLParam(r, "courseSlug")
	h.mu.RLock()
	c := h.courses[slug]
	var catDir string
	if c == nil {
		// maybe a catalog slug
		cat := h.catalogs[slug]
		if cat == nil { h.mu.RUnlock(); return "", "", false }
		catDir = cat.Dir
	} else {
		catDir = c.Dir
	}
	h.mu.RUnlock()
	src, _ := h.sourcesAll(r.Context())
	if _, has := src[catDir]; !has { return slug, "", false }
	return slug, filepath.Join(h.coursesDir, filepath.FromSlash(catDir)), true
}
```

checkout flow (course; catalog analogous — re-register via `loadAndRegisterCatalog`):
1. record `prevCommit, prevBranch`.
2. `CheckoutBranch(ctx, dir, branch, force)` — dirty && !force → 409 with git message.
3. Re-parse: `courseparser.LoadOne(h.coursesDir, slug)` (outside lock). On error: `git.CheckoutBranch(ctx, dir, prevBranch, true)` to roll back, re-parse (best effort), respond 422 with parser error.
4. On success: update `h.courses[c.Slug]` under `h.mu` (delete old slug key if slug changed between branches — slug==folder so it can't), `h.sources.Set(...)` new branch/commit.
5. Respond 200 `dto.GitImportResp`.

pull flow: `Dirty` && !force → 409; `PullFF` → re-parse (rollback: `reset --hard prevCommit` on failure → 422) → update sources → 200.

- [ ] **Step 4: Tests PASS.**

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat(git): branch switching, pull and status for imported courses"
```

---

### Task 6: Phase 1 frontend — GitHub settings, import dialog, course source panel

**Files:**
- Modify: `frontend/src/api/types.ts` (add `GitAuthStatus`, `GitImportResp`, `GitBranchesResp`, `GitStatusResp`)
- Modify: `frontend/src/api/client.ts` (add to `api` object)
- Create: `frontend/src/components/GitHubSettingsSection.tsx` (lazy-loaded like AISettingsSection)
- Modify: `frontend/src/components/SettingsPanel.tsx` (new tab `github`, lazy import + prefetch lines ~11-18, render ~1211)
- Modify: `frontend/src/pages/CoursesPage.tsx` (add «Импорт из GitHub» button + dialog next to existing upload UI)
- Modify: `frontend/src/pages/CoursePage.tsx` (source panel: branch select + Checkout + Pull + commit hash; only when `gitStatus.source != null`)

**Interfaces:**
- Consumes: endpoints from Tasks 3-5.
- Produces client methods:

```ts
gitAuth: () => get<GitAuthStatus>('/git/auth'),
gitSaveAuth: (body: { token?: string; username?: string }) => patch<GitAuthStatus>('/git/auth', body),
gitTestAuth: () => post<{ ok: boolean; login?: string; error?: string }>('/git/auth/test', {}),
gitImport: (body: { url: string; branch?: string }) => post<GitImportResp>('/git/import', body),
gitBranches: (slug: string) => get<GitBranchesResp>(`/courses/${slug}/git/branches`),
gitCheckout: (slug: string, branch: string, force = false) => post<GitImportResp>(`/courses/${slug}/git/checkout`, { branch, force }),
gitPull: (slug: string) => post<GitImportResp>(`/courses/${slug}/git/pull`, {}),
gitStatus: (slug: string) => get<GitStatusResp>(`/courses/${slug}/git/status`),
```

- [ ] **Step 1: Write tests first** — `frontend/src/components/GitHubSettingsSection.test.tsx`: renders token input; after save, shows masked token from mocked `api.gitAuth`; «Проверить» button surfaces `ok:false` error. Mock `../api/client` with `vi.mock` (follow existing component tests; if none mock api, use `vi.spyOn(api, ...)`).

- [ ] **Step 2: Verify FAIL** — `cd frontend && npx vitest run src/components/GitHubSettingsSection.test.tsx`

- [ ] **Step 3: Implement** —
  - Settings section: token field (type=password), Save (PATCH), Test (POST), Delete (PATCH `{token:""}`), status line «Токен сохранён: ghp_…mnop». Copy in Russian, styling via existing SettingsPanel classes.
  - CoursesPage dialog: URL input, optional branch input, «Импортировать» → `api.gitImport`; on error containing «private» → hint linking to Settings→GitHub; on success invalidate `courses`/`catalogs` queries.
  - CoursePage panel (top of page, compact row): `useQuery(['gitStatus', slug])` — branch name, short commit, `select` of branches (query `gitBranches` on open), buttons «Переключить» (confirm dialog when status.dirty: «Есть несохранённые изменения — переключиться принудительно?») and «Обновить» (pull). Mutations invalidate `['course', slug]`.

- [ ] **Step 4: Tests PASS** + `cd frontend && npm run lint` clean + `npx tsc -b` clean.

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "feat(ui): GitHub import dialog, branch panel and token settings"
```

**== PHASE 1 COMPLETE — checkpoint commit; continue to Phase 2 ==**

---

### Task 7: sync config repo

**Files:**
- Create: `backend/internal/infrastructure/repo/sync_config.go` (+ test)
- Create: `backend/internal/api/dto/sync.go`

**Interfaces:**
- Produces:
```go
type SyncTriggers struct {
	OnProgress    bool `json:"on_progress"`
	IntervalMin   int  `json:"interval_min"` // 0 = off
	OnStartupPull bool `json:"on_startup_pull"`
}
type SyncConfig struct {
	RemoteURL string       `json:"remote_url"`
	Branch    string       `json:"branch"`
	Triggers  SyncTriggers `json:"triggers"`
	LastSync  string       `json:"last_sync,omitempty"` // RFC3339
	Enabled   bool         `json:"enabled"`
}
func NewSyncConfigRepository(dataDir string) *SyncConfigRepository // sync_config.json
func (r *SyncConfigRepository) Load(ctx) (*SyncConfig, error) // (nil,nil) absent → caller uses defaults {Branch:"main"}
func (r *SyncConfigRepository) Save(ctx, *SyncConfig) error
```
DTOs (`dto/sync.go`): `SyncConfigResp` (= SyncConfig JSON), `PatchSyncConfigReq {RemoteURL, Branch *string; Triggers *SyncTriggers; Enabled *bool}`, `SyncStatusResp {Configured, Enabled, Syncing bool; LastSync, Branch, Commit string; PendingImports []string}`, `SyncHistoryItem {Commit, Author, Subject string; Time string}`, `SyncCommitFilesResp {Files []string}`, `RollbackReq {Commit string}`.

- [ ] **Step 1: Failing test** — round trip + absent-file defaults.
- [ ] **Step 2: FAIL.** [ ] **Step 3: Implement** (copy ai_config.go shape). [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit** `git add backend && git commit -m "feat(sync): config storage"`

---

### Task 8: sync engine — snapshot push, pull, status

**Files:**
- Create: `backend/internal/infrastructure/sync/sync.go` (+ `sync_test.go`)
- Modify: `backend/internal/di/di.go` (construct after handlers; inject into Handler)
- Modify: `backend/internal/api/handlers/handler.go` (`sync *sync.Engine` field)

**Interfaces:**
- Produces:
```go
type Engine struct {
	mu         sync.Mutex
	git        *git.Service
	cfgRepo    *repo.SyncConfigRepository
	sources    *repo.CourseSourcesRepository
	coursesDir string
	dataDir    string
	mirrorDir  string   // {dataDir}/sync/repo
	busy       atomic.Bool
	OnReload   func()   // set by DI: re-parse courses after pull (handlers reload callback)
}
func NewEngine(g *git.Service, cfgRepo *repo.SyncConfigRepository, sources *repo.CourseSourcesRepository, coursesDir, dataDir string) *Engine
func (e *Engine) Configured(ctx) (bool, error)
func (e *Engine) Push(ctx) error
func (e *Engine) Pull(ctx) error
func (e *Engine) Status(ctx) (*Status, error)   // {Syncing, LastSync, Branch, HeadCommit, PendingImports}
type Status struct {
	Syncing        bool     `json:"syncing"`
	LastSync       string   `json:"last_sync"`
	Branch         string   `json:"branch"`
	HeadCommit     string   `json:"head_commit"`
	PendingImports []string `json:"pending_imports"` // slugs in cloud sources.json missing locally
}
```

Push algorithm (all under `e.mu`, `busy` flag for Status):
1. cfg = Load; if !cfg.Enabled || cfg.RemoteURL == "" → error "sync not configured".
2. ensure mirror: if `mirrorDir/.git` missing → `MkdirAll` + `git.Init(mirrorDir, cfg.RemoteURL, cfg.Branch)` + initial empty commit (`CommitAll` "init vault").
3. `Fetch` (tolerate failure on brand-new empty remote: if fetch fails and remote has no branches, continue).
4. Align: if `origin/<branch>` exists → `git reset --hard origin/<branch>` + `git clean -fd` (via CheckoutBranch(dir, branch, force=true) after fetch); else stay on fresh init.
5. Wipe mirror content except `.git` (WalkDir top-level entries).
6. Copy: for each top-level dir in `coursesDir`: copy tree **excluding any nested `.git`** into `mirror/courses/<name>`; skip dirs listed as git-imported in sources (their content comes from their own remote — but still record them in `sources.json`). Copy `dataDir/progress/**` → `mirror/progress/**`. Write `sources.json` = `sources.All()` output.
7. `CommitAll(mirrorDir, fmt.Sprintf("sync: %s %s", hostname, time.Now().UTC().Format(time.RFC3339)))`.
8. `Push(mirrorDir, cfg.Branch)`; on rejection → retry steps 3-8 up to 3 times total; final failure → error "remote changed, retry later".
9. Save cfg.LastSync = now.

Pull algorithm:
1. ensure mirror + `Fetch`. If `origin/<branch>` missing → error "cloud vault is empty — push first".
2. `CheckoutBranch(mirrorDir, cfg.Branch, true)`.
3. Apply to coursesDir: for each `mirror/courses/<name>`: if local `coursesDir/<name>` exists and is a git-import (in sources) → **skip content, keep local clone**; else atomic replace (copy to `coursesDir/<name>.synctmp` then rename over). Local non-import course dirs absent from mirror → delete (cloud is source of truth).
4. Replace `dataDir/progress` from `mirror/progress` (same atomic pattern).
5. Load `mirror/sources.json` → merge into local sources (cloud entries for missing slugs recorded; `PendingImports` = cloud sources whose courseDir doesn't exist locally).
6. `e.OnReload()` — DI wires this to a Handler method that re-parses everything: `h.reloadCourses()` (new small method on Handler: `course.LoadAll(h.coursesDir)` under `h.mu`, replacing maps).
7. cfg.LastSync = now.

- [ ] **Step 1: Failing tests** (`sync_test.go`) — local "cloud" = second non-bare repo path acting as remote (git allows push to a non-bare repo only if `receive.denyCurrentBranch=ignore` — **set it**: in makeRemote, `git config receive.denyCurrentBranch ignore`; then real push works):

```go
func TestPushPullRoundTrip(t *testing.T) {
	// device A: engine with coursesDir containing a minimal valid course +
	// progress in dataDir; Push → cloud repo has courses/<slug>, progress/, sources.json
	// device B: fresh engine (own temp dirs) with same cloud; Pull → course dir
	// and progress materialize identically.
}

func TestPushOverwritesRemoteSnapshot(t *testing.T) {
	// push, modify local course file, push again → cloud HEAD contains new content,
	// history has 2+ sync commits.
}

func TestPullDeletesNonImportCourseAbsentFromCloud(t *testing.T) {
	// push (course A only); locally add course B (not pushed); Pull → B deleted.
}

func TestPullKeepsLocalGitImport(t *testing.T) {
	// local course with .git (simulated import) + sources entry; push; wipe mirror
	// courses/<slug> from cloud (simulate another device without it) via direct
	// commit; Pull → local .git dir intact.
}
```

- [ ] **Step 2: FAIL.** [ ] **Step 3: Implement** (copyTree helper with skip-.git; atomic dir replace via rename; `hostname` from `os.Hostname`). [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit** `git commit -m "feat(sync): snapshot push/pull engine"`

---

### Task 9: sync HTTP endpoints + push race retry test

**Files:**
- Create: `backend/internal/api/handlers/sync.go` (+ test)
- Modify: `routes.go`, `handler.go` (engine access), `di.go` (wire engine into handlers.New, set `engine.OnReload = h.reloadCourses`)

**Interfaces:**
- Routes:
  - `GET /sync/config` → `dto.SyncConfigResp`
  - `PATCH /sync/config` body `dto.PatchSyncConfigReq` → `dto.SyncConfigResp` (changing RemoteURL/Branch wipes mirror dir so next push re-clones)
  - `POST /sync/push`, `POST /sync/pull` → 200 `dto.SyncStatusResp`; 409 if `busy`; 503 git missing; 400 not configured
  - `GET /sync/status` → `dto.SyncStatusResp`
  - `GET /sync/history?limit=50` → `[]dto.SyncHistoryItem`
  - `GET /sync/history/{commit}` → `dto.SyncCommitFilesResp` (validate commit is hex, reject anything else)
  - `POST /sync/rollback` body `dto.RollbackReq` → 200 status
  - `POST /sync/restore-imports` → `{restored: [slugs]}` — for each PendingImport: Clone into coursesDir/<dir> (skip if exists), record source, then `h.reloadCourses()`.

Rollback (engine method `Rollback(ctx, commit)`): under mu → `Fetch` → `git checkout <commit> -- .` in mirror (content of that commit on top of current branch) → copy-back exactly like Pull steps 3-6 → `CommitAll("rollback to <short>")` → `Push` (retry ×3 like Push).

- [ ] **Step 1: Failing tests** — handler-level (httptest): config PATCH persists; push→history shows commit; rollback to first commit restores old file content locally and adds a new commit; **race**: push once; from test code commit+push directly to the cloud repo (simulating device B); engine.Push again → succeeds with 2 sync commits after the foreign one... (foreign commit gets overwritten by snapshot — assert final cloud content equals device A state and history contains the foreign commit).

- [ ] **Step 2: FAIL.** [ ] **Step 3: Implement.** [ ] **Step 4: PASS** (`go test ./internal/api/handlers/ ./internal/infrastructure/sync/`).
- [ ] **Step 5: Commit** `git commit -m "feat(sync): HTTP API — config, push/pull, history, rollback, restore-imports"`

---

### Task 10: triggers (on-progress debounce, interval, startup pull)

**Files:**
- Modify: `backend/internal/application/service/progress.go` (add `onChange func()` field + `SetOnChange`)
- Modify: `backend/internal/infrastructure/sync/sync.go` (add `Start(ctx)/Stop()` for interval ticker + debounce timer)
- Modify: `backend/internal/di/di.go` (wire: `ps.SetOnChange(engine.NotifyProgress)`; if cfg.Enabled && OnStartupPull → `go engine.Pull(bg)`; start engine timers; stop on shutdown)

**Interfaces:**
- Produces: `func (e *Engine) NotifyProgress()` — if cfg.Enabled && Triggers.OnProgress: reset 30s timer; on fire → `go e.Push(context.Background())` (errors logged only). `func (e *Engine) Start(ctx context.Context)` — interval ticker when IntervalMin>0. `func (e *Engine) Stop()` — stops timer/ticker.

- [ ] **Step 1: Failing test** — `progress_test.go` (service): `SetOnChange(f)` → MarkDone calls f. `sync_test.go`: NotifyProgress with a fake clock is overkill — instead unit-test the debounce helper: `newDebouncer(d, fn)` with d=10ms; two rapid Notify calls → fn runs once (wait 50ms, count==1).
- [ ] **Step 2: FAIL.** [ ] **Step 3: Implement** (debouncer: `time.AfterFunc` + Reset under mutex — stdlib only). [ ] **Step 4: PASS.**
- [ ] **Step 5: Commit** `git commit -m "feat(sync): progress/interval/startup triggers"`

---

### Task 11: Phase 2 frontend — Cloud settings section

**Files:**
- Modify: `frontend/src/api/types.ts`, `frontend/src/api/client.ts`
- Create: `frontend/src/components/CloudSettingsSection.tsx` (+ test)
- Modify: `frontend/src/components/SettingsPanel.tsx` (tab `cloud` — combine GitHub token + cloud vault in one «GitHub и облако» tab? **Decision: one tab `github`** containing GitHubSettingsSection + CloudSettingsSection stacked — fewer tabs, shared auth context.)

**Interfaces:**
- Client:

```ts
syncConfig: () => get<SyncConfigResp>('/sync/config'),
syncSaveConfig: (body: PatchSyncConfigReq) => patch<SyncConfigResp>('/sync/config', body),
syncPush: () => post<SyncStatusResp>('/sync/push', {}),
syncPull: () => post<SyncStatusResp>('/sync/pull', {}),
syncStatus: () => get<SyncStatusResp>('/sync/status'),
syncHistory: (limit = 50) => get<SyncHistoryItem[]>(`/sync/history?limit=${limit}`),
syncCommitFiles: (commit: string) => get<{ files: string[] }>(`/sync/history/${commit}`),
syncRollback: (commit: string) => post<SyncStatusResp>('/sync/rollback', { commit }),
syncRestoreImports: () => post<{ restored: string[] }>('/sync/restore-imports', {}),
```

- [ ] **Step 1: Failing test** — `CloudSettingsSection.test.tsx`: renders remote URL input + triggers; «Синхронизировать» calls `api.syncPush`; history list renders mocked commits; rollback button asks confirmation (in-page, no `confirm()` — follow existing dialog pattern in SettingsPanel).
- [ ] **Step 2: FAIL.**
- [ ] **Step 3: Implement** — sections: (1) Настройка: remote URL, branch, enable toggle, Save; (2) Триггеры: checkboxes + interval input; (3) Статус: last sync, branch, «Синхронизировать» / «Скачать» buttons, spinner while status.syncing (poll `syncStatus` every 3s while syncing); (4) История: commit list (short hash, date, subject), expand → files, «Откатиться» with confirm; (5) «Восстановить импортированные курсы» button when pendingImports non-empty.
- [ ] **Step 4: PASS** + lint + `npx tsc -b` clean.
- [ ] **Step 5: Commit** `git commit -m "feat(ui): cloud sync settings, history and rollback"`

---

### Task 12: swagger, full build, docs

**Files:**
- Modify: `AGENTS.md` (Data storage table: progress new location, `git_auth.json`, `sync_config.json`, `course_sources.json`, `sync/repo/`; new API group)
- Generated: `backend/docs/*` (swagger)

- [ ] **Step 1:** `cd backend && make swagger` — verify no errors.
- [ ] **Step 2:** `cd backend && make test` — full backend suite green.
- [ ] **Step 3:** `cd frontend && npx vitest run && npm run lint`.
- [ ] **Step 4:** Full build: `./scripts/build.ps1` (Windows) — binary produced, embedded frontend OK.
- [ ] **Step 5: Manual smoke** — run binary, open UI: import a public GitHub course repo if one exists (else local test repo), toggle branch, set token (fake) and see mask, configure sync to a local bare repo path via... **remote_url is validated as github.com in import but NOT in sync config** — sync accepts any git remote URL (local paths allowed → testable). Push/pull/rollback round trip through UI.
- [ ] **Step 6: Update AGENTS.md + commit** `git commit -m "docs: swagger regen, AGENTS.md storage table for git/sync"`

**== PHASE 2 COMPLETE — report to user, await push approval ==**
