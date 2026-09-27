package sync

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/infrastructure/git"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
)

func gitAvailable(t *testing.T) {
	t.Helper()
	if exec.Command("git", "--version").Run() != nil {
		t.Skip("git not installed")
	}
}

// rawGit runs git directly (test-side only, never through the engine).
func rawGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// makeCloud creates a non-bare repo acting as the cloud vault. Pushing to a
// checked-out branch requires receive.denyCurrentBranch=ignore; the working
// tree stays stale, so tests inspect content via ls-tree/show.
func makeCloud(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cloud")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	rawGit(t, dir, "init", "-b", "main")
	rawGit(t, dir, "config", "receive.denyCurrentBranch", "ignore")
	rawGit(t, dir, "config", "core.autocrlf", "false")
	return dir
}

func cloudFiles(t *testing.T, cloud string) []string {
	t.Helper()
	out := rawGit(t, cloud, "ls-tree", "-r", "--name-only", "main")
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files
}

func cloudFile(t *testing.T, cloud, path string) string {
	t.Helper()
	return rawGit(t, cloud, "show", "main:"+path)
}

type testEnv struct {
	engine     *Engine
	coursesDir string
	dataDir    string
	cfgRepo    *repo.SyncConfigRepository
	sources    *repo.CourseSourcesRepository
}

func newTestEnv(t *testing.T, cloud string) *testEnv {
	t.Helper()
	base := t.TempDir()
	coursesDir := filepath.Join(base, "courses")
	dataDir := filepath.Join(base, "data")
	for _, d := range []string{coursesDir, dataDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfgRepo := repo.NewSyncConfigRepository(dataDir)
	cfg := &repo.SyncConfig{RemoteURL: cloud, Branch: "main", Enabled: true}
	if err := cfgRepo.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	sources := repo.NewCourseSourcesRepository(dataDir)
	// Pre-create the mirror with autocrlf disabled so Windows CRLF conversion
	// doesn't alter content between push and pull; ensureMirror reuses it.
	mirror := filepath.Join(dataDir, "sync", "repo")
	if err := os.MkdirAll(mirror, 0755); err != nil {
		t.Fatal(err)
	}
	rawGit(t, mirror, "init", "-b", "main")
	rawGit(t, mirror, "remote", "add", "origin", "--", cloud)
	rawGit(t, mirror, "config", "core.autocrlf", "false")
	e := NewEngine(git.NewService(), cfgRepo, sources, coursesDir, dataDir)
	return &testEnv{engine: e, coursesDir: coursesDir, dataDir: dataDir, cfgRepo: cfgRepo, sources: sources}
}

func writeCourse(t *testing.T, coursesDir, slug, body string) {
	t.Helper()
	dir := filepath.Join(coursesDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "course.yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeProgress(t *testing.T, dataDir, courseDir, content string) {
	t.Helper()
	dir := filepath.Join(dataDir, "progress", filepath.FromSlash(courseDir))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestConfiguredAndPushRequiresConfig(t *testing.T) {
	gitAvailable(t)
	env := newTestEnv(t, makeCloud(t))
	ctx := context.Background()

	ok, err := env.engine.Configured(ctx)
	if err != nil || !ok {
		t.Fatalf("configured = %v, %v", ok, err)
	}
	if err := env.cfgRepo.Save(ctx, &repo.SyncConfig{RemoteURL: "", Branch: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	ok, _ = env.engine.Configured(ctx)
	if ok {
		t.Fatal("configured with empty remote")
	}
	if err := env.engine.Push(ctx); err == nil {
		t.Fatal("push without remote must fail")
	}
}

func TestPushPullRoundTrip(t *testing.T) {
	gitAvailable(t)
	cloud := makeCloud(t)
	ctx := context.Background()

	// device A
	envA := newTestEnv(t, cloud)
	writeCourse(t, envA.coursesDir, "go-basics", "slug: go-basics\ntitle: Go Basics\n")
	writeProgress(t, envA.dataDir, "go-basics", `{"course_slug":"go-basics","completed_tasks":{"t1":true}}`)
	if err := envA.engine.Push(ctx); err != nil {
		t.Fatalf("push: %v", err)
	}

	files := strings.Join(cloudFiles(t, cloud), "\n")
	for _, want := range []string{
		"courses/go-basics/course.yaml",
		"progress/go-basics/progress.json",
		"sources.json",
	} {
		if !strings.Contains(files, want) {
			t.Fatalf("cloud missing %s:\n%s", want, files)
		}
	}
	cfgA, _ := envA.cfgRepo.Load(ctx)
	if cfgA.LastSync == "" {
		t.Fatal("LastSync not recorded after push")
	}

	// device B: fresh dirs, same cloud
	envB := newTestEnv(t, cloud)
	reloaded := false
	envB.engine.OnReload = func() { reloaded = true }
	if err := envB.engine.Pull(ctx); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if got := mustRead(t, filepath.Join(envB.coursesDir, "go-basics", "course.yaml")); got != "slug: go-basics\ntitle: Go Basics\n" {
		t.Fatalf("course content mismatch: %q", got)
	}
	if got := mustRead(t, filepath.Join(envB.dataDir, "progress", "go-basics", "progress.json")); !strings.Contains(got, "t1") {
		t.Fatalf("progress mismatch: %q", got)
	}
	if !reloaded {
		t.Fatal("OnReload not called after pull")
	}
	st, err := envB.engine.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || st.HeadCommit == "" || st.Syncing {
		t.Fatalf("status = %+v", st)
	}
}

func TestPushOverwritesRemoteSnapshot(t *testing.T) {
	gitAvailable(t)
	cloud := makeCloud(t)
	ctx := context.Background()
	env := newTestEnv(t, cloud)

	writeCourse(t, env.coursesDir, "c1", "version: one\n")
	if err := env.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}
	writeCourse(t, env.coursesDir, "c1", "version: two\n")
	if err := env.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}
	if got := cloudFile(t, cloud, "courses/c1/course.yaml"); got != "version: two\n" {
		t.Fatalf("cloud content = %q", got)
	}
	mirror := filepath.Join(env.dataDir, "sync", "repo")
	log, err := env.engine.git.Log(ctx, mirror, 20)
	if err != nil {
		t.Fatal(err)
	}
	syncCommits := 0
	for _, c := range log {
		if strings.HasPrefix(c.Subject, "sync:") {
			syncCommits++
		}
	}
	if syncCommits < 2 {
		t.Fatalf("expected 2+ sync commits, got %d in %+v", syncCommits, log)
	}
}

func TestPushSkipsExcludedAndImported(t *testing.T) {
	gitAvailable(t)
	cloud := makeCloud(t)
	ctx := context.Background()
	env := newTestEnv(t, cloud)

	writeCourse(t, env.coursesDir, "normal", "a\n")
	writeCourse(t, env.coursesDir, "excluded", "b\n")
	writeCourse(t, env.coursesDir, "imported", "c\n")

	cfg, _ := env.cfgRepo.Load(ctx)
	cfg.Exclude = []string{"excluded"}
	if err := env.cfgRepo.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if err := env.sources.Set(ctx, "imported", repo.CourseSource{Repo: "https://github.com/x/y", Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := env.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}

	files := strings.Join(cloudFiles(t, cloud), "\n")
	if !strings.Contains(files, "courses/normal/course.yaml") {
		t.Fatalf("normal course missing:\n%s", files)
	}
	if strings.Contains(files, "courses/excluded") || strings.Contains(files, "courses/imported") {
		t.Fatalf("excluded/imported content leaked to cloud:\n%s", files)
	}
	// imported course is still recorded in cloud sources.json
	var srcs map[string]repo.CourseSource
	if err := json.Unmarshal([]byte(cloudFile(t, cloud, "sources.json")), &srcs); err != nil {
		t.Fatal(err)
	}
	if _, ok := srcs["imported"]; !ok {
		t.Fatalf("imported source not recorded: %v", srcs)
	}
}

func TestPullKeepsLocalCourseAbsentFromCloud(t *testing.T) {
	gitAvailable(t)
	cloud := makeCloud(t)
	ctx := context.Background()

	env := newTestEnv(t, cloud)
	writeCourse(t, env.coursesDir, "A", "cloud course\n")
	if err := env.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}
	// local-only course B and an excluded course C whose cloud copy differs
	writeCourse(t, env.coursesDir, "B", "local only\n")
	writeCourse(t, env.coursesDir, "C", "local C\n")
	cfg, _ := env.cfgRepo.Load(ctx)
	cfg.Exclude = []string{"C"}
	if err := env.cfgRepo.Save(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	// sneak a different C into the cloud via a second device that syncs C
	env2 := newTestEnv(t, cloud)
	if err := env2.engine.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	writeCourse(t, env2.coursesDir, "C", "cloud C\n")
	if err := env2.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}

	if err := env.engine.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(env.coursesDir, "B", "course.yaml")); got != "local only\n" {
		t.Fatalf("course B lost: %q", got)
	}
	if got := mustRead(t, filepath.Join(env.coursesDir, "C", "course.yaml")); got != "local C\n" {
		t.Fatalf("excluded course C overwritten: %q", got)
	}
	if got := mustRead(t, filepath.Join(env.coursesDir, "A", "course.yaml")); got != "cloud course\n" {
		t.Fatalf("course A lost: %q", got)
	}
}

func TestPullKeepsLocalGitImport(t *testing.T) {
	gitAvailable(t)
	cloud := makeCloud(t)
	ctx := context.Background()

	env := newTestEnv(t, cloud)
	// simulate a git-imported course: own .git + sources entry
	importedDir := filepath.Join(env.coursesDir, "imp")
	if err := os.MkdirAll(importedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(importedDir, "course.yaml"), []byte("local import\n"), 0644); err != nil {
		t.Fatal(err)
	}
	rawGit(t, importedDir, "init", "-b", "main")
	rawGit(t, importedDir, "add", "-A")
	rawGit(t, importedDir, "commit", "-m", "import init")
	if err := env.sources.Set(ctx, "imp", repo.CourseSource{Repo: "https://github.com/x/imp", Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	writeCourse(t, env.coursesDir, "plain", "plain\n")
	if err := env.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}

	// another device wipes courses/imp from the cloud (it never had the import)
	env2 := newTestEnv(t, cloud)
	if err := env2.engine.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	if err := env2.engine.Push(ctx); err != nil { // env2 has no "imp" dir at all
		t.Fatal(err)
	}

	if err := env.engine.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(importedDir, ".git")); err != nil {
		t.Fatalf(".git of import destroyed: %v", err)
	}
	if got := mustRead(t, filepath.Join(importedDir, "course.yaml")); got != "local import\n" {
		t.Fatalf("import content changed: %q", got)
	}
}

func TestPullEmptyCloudErrors(t *testing.T) {
	gitAvailable(t)
	env := newTestEnv(t, makeCloud(t))
	err := env.engine.Pull(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("pull from empty cloud = %v", err)
	}
}

func TestPullRecordsPendingImports(t *testing.T) {
	gitAvailable(t)
	cloud := makeCloud(t)
	ctx := context.Background()

	// device A has a git-imported course (recorded in sources, content skipped)
	envA := newTestEnv(t, cloud)
	writeCourse(t, envA.coursesDir, "plain", "p\n")
	if err := envA.sources.Set(ctx, "imp", repo.CourseSource{Repo: "https://github.com/x/imp", Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := envA.engine.Push(ctx); err != nil {
		t.Fatal(err)
	}

	// device B pulls: "imp" is in cloud sources but absent locally
	envB := newTestEnv(t, cloud)
	if err := envB.engine.Pull(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := envB.engine.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range st.PendingImports {
		if s == "imp" {
			found = true
		}
	}
	if !found {
		t.Fatalf("pending imports = %v", st.PendingImports)
	}
	// the cloud source entry merged into local sources
	srcs, _ := envB.sources.All(ctx)
	if srcs["imp"].Repo != "https://github.com/x/imp" {
		t.Fatalf("sources not merged: %+v", srcs)
	}
}
