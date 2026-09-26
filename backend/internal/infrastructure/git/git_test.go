package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitAvailable(t *testing.T) {
	t.Helper()
	if exec.Command("git", "--version").Run() != nil {
		t.Skip("git not installed")
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// makeRemote creates a non-bare repo with branches main and dev to clone/fetch from.
func makeRemote(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")
	gitRun(t, dir, "checkout", "-b", "dev")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dev"), 0644)
	gitRun(t, dir, "commit", "-am", "dev change")
	gitRun(t, dir, "checkout", "main")
	return dir
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
	br, err := s.CurrentBranch(ctx, dir)
	if err != nil || br != "main" {
		t.Fatalf("branch = %q, err %v", br, err)
	}
	branches, err := s.RemoteBranches(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("branches = %v", branches)
	}
	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}
	if !found["main"] || !found["dev"] {
		t.Fatalf("expected main+dev, got %v", branches)
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
	// dirty tree refuses checkout without force
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local edit"), 0644)
	if err := s.CheckoutBranch(ctx, dir, "main", false); err == nil {
		t.Fatal("checkout must refuse on dirty tree without force")
	}
	if err := s.CheckoutBranch(ctx, dir, "main", true); err != nil {
		t.Fatalf("force checkout: %v", err)
	}
	// ff-only pull: commit a new file directly on remote main, then pull
	os.WriteFile(filepath.Join(remote, "b.txt"), []byte("new"), 0644)
	gitRun(t, remote, "add", "-A")
	gitRun(t, remote, "commit", "-m", "remote change")
	if err := s.PullFF(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("pull did not fetch new file: %v", err)
	}
	log, err := s.Log(ctx, dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) < 2 || log[0].Subject == "" {
		t.Fatalf("log wrong: %+v", log)
	}
	if log[0].Subject != "remote change" {
		t.Fatalf("latest commit = %q", log[0].Subject)
	}
	head, err := s.HeadCommit(ctx, dir)
	if err != nil || len(head) != 40 {
		t.Fatalf("head = %q err %v", head, err)
	}
}

func TestCheckoutToBranchSpecific(t *testing.T) {
	gitAvailable(t)
	remote := makeRemote(t)
	s := NewService()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "clone")
	if err := s.Clone(ctx, remote, dir, "dev"); err != nil {
		t.Fatal(err)
	}
	br, _ := s.CurrentBranch(ctx, dir)
	if br != "dev" {
		t.Fatalf("clone -b dev landed on %q", br)
	}
}

func TestTokenPassedViaEnvOnly(t *testing.T) {
	gitAvailable(t)
	s := NewService()
	s.SetToken("ghp_secret123")
	if s.Token() != "ghp_secret123" {
		t.Fatal("Token() mismatch")
	}
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

func TestCommitAllAndInit(t *testing.T) {
	gitAvailable(t)
	s := NewService()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "mirror")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// remote to push to
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitRun(t, filepath.Dir(remote), "init", "--bare", filepath.Base(remote))
	if err := s.Init(ctx, dir, remote, "main"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hi"), 0644)
	if err := s.CommitAll(ctx, dir, "first"); err != nil {
		t.Fatal(err)
	}
	if err := s.Push(ctx, dir, "main"); err != nil {
		t.Fatal(err)
	}
	// CommitAll with no changes is a no-op, not an error
	if err := s.CommitAll(ctx, dir, "empty"); err != nil {
		t.Fatalf("CommitAll on clean tree: %v", err)
	}
	files, err := s.CommitFiles(ctx, dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "x.txt" {
		t.Fatalf("CommitFiles = %v", files)
	}
}

func TestCheckoutPathsRollback(t *testing.T) {
	gitAvailable(t)
	s := NewService()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "r")
	os.MkdirAll(dir, 0755)
	gitRun(t, dir, "init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("v1"), 0644)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "v1")
	first, _ := s.HeadCommit(ctx, dir)
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("v2"), 0644)
	gitRun(t, dir, "commit", "-am", "v2")
	// restore working tree content from first commit
	if err := s.CheckoutPaths(ctx, dir, first); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(data) != "v1" {
		t.Fatalf("rollback content = %q", data)
	}
}
