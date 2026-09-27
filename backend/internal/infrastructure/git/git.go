// Package git wraps the system git CLI. The GitHub token is injected through
// GIT_CONFIG_* environment variables so it never lands in process args (visible
// to other processes), in .git/config, or in a credential store.
package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Error carries a git failure. Stderr is the trimmed git error output.
type Error struct {
	Stderr   string
	ExitCode int
}

func (e *Error) Error() string { return e.Stderr }

// validateRef rejects empty and dash-prefixed values so an untrusted branch or
// URL can never be parsed as a git flag (argv flag smuggling).
func validateRef(v string) error {
	if v == "" {
		return errors.New("empty value")
	}
	if strings.HasPrefix(v, "-") {
		return errors.New("value must not start with a dash")
	}
	return nil
}

// validateBranch rejects empty names, dash prefixes and anything containing a
// ".." traversal sequence or an option-injection colon.
func validateBranch(b string) error {
	if err := validateRef(b); err != nil {
		return errors.New("invalid branch name")
	}
	if strings.Contains(b, "..") || strings.ContainsAny(b, ":^~[ \\") {
		return errors.New("invalid branch name")
	}
	return nil
}

// Commit is one line of git log output.
type Commit struct {
	Hash    string
	Time    time.Time
	Author  string
	Subject string
}

// Service serializes git invocations. Every public method holds mu for the
// whole command sequence, so multi-step operations (checkout = fetch + reset +
// checkout) are atomic with respect to other calls on the same Service.
//
// ponytail: one global mutex serializes all repos; per-repo locks if a fleet of
// concurrent syncs ever contends.
type Service struct {
	mu           sync.Mutex
	token        string
	timeout      time.Duration
	cloneTimeout time.Duration
	availOnce    sync.Once
	avail        bool
}

func NewService() *Service {
	return &Service{timeout: 60 * time.Second, cloneTimeout: 5 * time.Minute}
}

func (s *Service) SetToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
}

func (s *Service) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// Available reports whether the git CLI is on PATH, probed once and cached.
func (s *Service) Available() bool {
	s.availOnce.Do(func() {
		s.avail = exec.Command("git", "--version").Run() == nil
	})
	return s.avail
}

// runCmd executes git without locking; the caller must hold s.mu. dir is the
// working directory ("" = inherit the process cwd, used before a clone target
// exists).
func (s *Service) runCmd(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	env := os.Environ()
	env = append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=CourseForge", "GIT_AUTHOR_EMAIL=courseforge@local",
		"GIT_COMMITTER_NAME=CourseForge", "GIT_COMMITTER_EMAIL=courseforge@local")
	count := 1
	env = append(env,
		"GIT_CONFIG_KEY_0=protocol.ext.allow", "GIT_CONFIG_VALUE_0=never")
	if s.token != "" {
		count = 2
		env = append(env,
			"GIT_CONFIG_KEY_1=http.https://github.com/.extraHeader",
			"GIT_CONFIG_VALUE_1=Authorization: Bearer "+s.token)
	}
	env = append(env, "GIT_CONFIG_COUNT="+strconv.Itoa(count))
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

// validateURL rejects dash-prefixed URLs and the ext:: transport, which runs
// arbitrary commands. Protocol pinning to https://github.com happens in the
// handler layer; this stays transport-agnostic so local test remotes work.
func validateURL(u string) error {
	if err := validateRef(u); err != nil {
		return errors.New("invalid repository url")
	}
	if strings.HasPrefix(strings.ToLower(u), "ext::") {
		return errors.New("invalid repository url")
	}
	return nil
}

// Clone clones url into dir. branch may be "" for the remote default.
func (s *Service) Clone(ctx context.Context, url, dir, branch string) error {
	if err := validateURL(url); err != nil {
		return err
	}
	if branch != "" {
		if err := validateBranch(branch); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, "--", url, dir)
	_, err := s.runCmd(ctx, filepath.Dir(dir), s.cloneTimeout, args...)
	return err
}

func (s *Service) Fetch(ctx context.Context, dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.runCmd(ctx, dir, s.timeout, "fetch", "origin")
	return err
}

// RemoteBranches lists origin's branches without the "origin/" prefix.
func (s *Service) RemoteBranches(ctx context.Context, dir string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.runCmd(ctx, dir, s.timeout, "branch", "-r")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "->") {
			continue
		}
		if i := strings.Index(line, "/"); i >= 0 {
			line = line[i+1:]
		}
		branches = append(branches, line)
	}
	return branches, nil
}

func (s *Service) CurrentBranch(ctx context.Context, dir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentBranchLocked(ctx, dir)
}

func (s *Service) currentBranchLocked(ctx context.Context, dir string) (string, error) {
	out, err := s.runCmd(ctx, dir, s.timeout, "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

func (s *Service) HeadCommit(ctx context.Context, dir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.runCmd(ctx, dir, s.timeout, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// CheckoutBranch fetches and hard-aligns dir onto origin/branch. Without force
// it refuses when the working tree is dirty (uncommitted local edits).
func (s *Service) CheckoutBranch(ctx context.Context, dir, branch string, force bool) error {
	if err := validateBranch(branch); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.runCmd(ctx, dir, s.timeout, "fetch", "origin"); err != nil {
		return err
	}
	if force {
		_, _ = s.runCmd(ctx, dir, s.timeout, "clean", "-fd")
		if _, err := s.runCmd(ctx, dir, s.timeout, "reset", "--hard"); err != nil {
			return err
		}
	} else {
		dirty, err := s.dirtyLocked(ctx, dir)
		if err != nil {
			return err
		}
		if dirty {
			return &Error{Stderr: "working tree has uncommitted changes; commit or discard them first"}
		}
	}
	_, err := s.runCmd(ctx, dir, s.timeout, "checkout", "-B", branch, "origin/"+branch)
	return err
}

// PullFF fetches and fast-forwards the current branch. A diverged branch is an
// error (never a merge commit, never a force).
func (s *Service) PullFF(ctx context.Context, dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.runCmd(ctx, dir, s.timeout, "fetch", "origin"); err != nil {
		return err
	}
	branch, err := s.currentBranchLocked(ctx, dir)
	if err != nil {
		return err
	}
	_, err = s.runCmd(ctx, dir, s.timeout, "merge", "--ff-only", "origin/"+branch)
	return err
}

// ResetHard points the current branch at ref and discards all working-tree
// changes (reset --hard ref + clean -fd).
func (s *Service) ResetHard(ctx context.Context, dir, ref string) error {
	if err := validateRef(ref); err != nil {
		return errors.New("invalid commit ref")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.runCmd(ctx, dir, s.timeout, "reset", "--hard", ref); err != nil {
		return err
	}
	_, err := s.runCmd(ctx, dir, s.timeout, "clean", "-fd")
	return err
}

// Dirty reports whether the working tree has uncommitted changes.
func (s *Service) Dirty(ctx context.Context, dir string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirtyLocked(ctx, dir)
}

func (s *Service) dirtyLocked(ctx context.Context, dir string) (bool, error) {
	out, err := s.runCmd(ctx, dir, s.timeout, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Init creates a repo on branch with origin pointed at remoteURL.
func (s *Service) Init(ctx context.Context, dir, remoteURL, branch string) error {
	if err := validateURL(remoteURL); err != nil {
		return err
	}
	if err := validateBranch(branch); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if _, err := s.runCmd(ctx, dir, s.timeout, "init", "-b", branch); err != nil {
		return err
	}
	_, err := s.runCmd(ctx, dir, s.timeout, "remote", "add", "origin", "--", remoteURL)
	return err
}

// CommitAll stages everything and commits; a clean tree is a no-op, not an error.
func (s *Service) CommitAll(ctx context.Context, dir, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.runCmd(ctx, dir, s.timeout, "add", "-A"); err != nil {
		return err
	}
	status, err := s.runCmd(ctx, dir, s.timeout, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil // nothing to commit
	}
	_, err = s.runCmd(ctx, dir, s.timeout, "commit", "-m", message)
	return err
}

// SetLocalConfig writes a repo-local config value (e.g. core.autocrlf).
func (s *Service) SetLocalConfig(ctx context.Context, dir, key, value string) error {
	if err := validateRef(key); err != nil {
		return errors.New("invalid config key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.runCmd(ctx, dir, s.timeout, "config", key, value)
	return err
}

// Stash pushes working-tree changes onto the stash stack. A clean tree is a
// no-op, not an error. Named so a conflicting pop can be traced back.
func (s *Service) Stash(ctx context.Context, dir, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirty, err := s.dirtyLocked(ctx, dir)
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	_, err = s.runCmd(ctx, dir, s.timeout, "stash", "push", "-m", name)
	return err
}

// StashPop applies and drops the newest stash entry. On a merge conflict the
// apply fails AND git keeps the entry — data is never lost; the caller resets
// the tree and reports that the changes are recoverable from the stash.
func (s *Service) StashPop(ctx context.Context, dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.runCmd(ctx, dir, s.timeout, "stash", "pop")
	return err
}

// StashList returns `git stash list` output ("" when empty).
func (s *Service) StashList(ctx context.Context, dir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.runCmd(ctx, dir, s.timeout, "stash", "list")
	return strings.TrimSpace(out), err
}

// CredentialHelper reports the configured credential.helper values for dir
// (system + global + repo-local, in git's resolution order). Empty = git will
// prompt or fail on private repos with no token configured. The values are
// shown in Settings so users understand where their access comes from — git
// silently uses OS credential stores, which is surprising otherwise.
func (s *Service) CredentialHelper(ctx context.Context, dir string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		dir = "" // no repo context: read the user-level config only
	}
	out, err := s.runCmd(ctx, dir, s.timeout, "config", "--show-origin", "--get-all", "credential.helper")
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && ge.ExitCode == 1 {
			return nil, nil // unset — nothing configured
		}
		return nil, err
	}
	var helpers []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// --show-origin prefixes "file:C:/path\tvalue" — keep a readable
		// "scope: value" form
		if i := strings.Index(line, "\t"); i >= 0 {
			scope := line[:i]
			value := line[i+1:]
			scope = strings.TrimPrefix(scope, "file:")
			helpers = append(helpers, simplifyConfigScope(scope)+": "+value)
		} else {
			helpers = append(helpers, line)
		}
	}
	return helpers, nil
}

// simplifyConfigScope shortens a config-file origin to system/global/local.
func simplifyConfigScope(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(lower, "/etc/gitconfig") || strings.Contains(lower, "programdata"):
		return "system"
	case strings.Contains(lower, ".gitconfig") || strings.Contains(lower, ".config/git"):
		return "global"
	case strings.Contains(lower, ".git/config") || strings.HasSuffix(lower, "config"):
		return "local"
	}
	return path
}

// WorkingTreeDiff returns `git diff` (unstaged + staged, tracked files) of
// the working tree against HEAD, plus `--porcelain` status entries including
// untracked files. Patches are raw unified diff text; the UI renders them.
func (s *Service) WorkingTreeDiff(ctx context.Context, dir string) ([]DiffFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.runCmd(ctx, dir, s.timeout, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, err
	}
	var files []DiffFile
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		code := strings.TrimSpace(line[:2])
		path := strings.Trim(strings.TrimSpace(line[3:]), `"`)
		st := "M"
		switch {
		case strings.Contains(code, "D"):
			st = "D"
		case code == "??":
			st = "A" // untracked = new file for the user
		case strings.Contains(code, "A"):
			st = "A"
		case strings.Contains(code, "R"):
			st = "R"
		}
		df := DiffFile{Path: path, Status: st}
		// patch only for tracked modifications; untracked have no diff hunks
		if st == "M" || st == "D" {
			out, err := s.runCmd(ctx, dir, s.timeout, "diff", "--", path)
			if err == nil {
				df.Hunks = splitHunks(out)
			}
		}
		files = append(files, df)
	}
	return files, nil
}

// DiffFile is one changed file with its unified-diff hunks.
type DiffFile struct {
	Path   string
	Status string // M / D / A / R
	Hunks  []string
}

// splitHunks splits a unified diff into per-hunk strings, dropping the
// file-header lines (---/+++) so each hunk starts with @@.
func splitHunks(diff string) []string {
	var hunks []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			hunks = append(hunks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			flush()
			cur = append(cur, line)
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			continue // header noise
		case len(cur) > 0:
			cur = append(cur, line)
		}
	}
	flush()
	return hunks
}

// Push pushes branch to origin. Never forces.
func (s *Service) Push(ctx context.Context, dir, branch string) error {
	if err := validateBranch(branch); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.runCmd(ctx, dir, s.timeout, "push", "origin", branch)
	return err
}

// Log returns up to limit commits, newest first.
func (s *Service) Log(ctx context.Context, dir string, limit int) ([]Commit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.runCmd(ctx, dir, s.timeout, "log",
		"-n", strconv.Itoa(limit), "--format=%H%x1f%at%x1f%an%x1f%s")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\x1f")
		if len(f) != 4 {
			continue
		}
		secs, _ := strconv.ParseInt(f[1], 10, 64)
		commits = append(commits, Commit{
			Hash:    f[0],
			Time:    time.Unix(secs, 0),
			Author:  f[2],
			Subject: f[3],
		})
	}
	return commits, nil
}

// CommitFiles lists the files touched by a commit (or ref like HEAD).
func (s *Service) CommitFiles(ctx context.Context, dir, hash string) ([]string, error) {
	if err := validateRef(hash); err != nil {
		return nil, errors.New("invalid commit ref")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.runCmd(ctx, dir, s.timeout, "show", "--name-only", "--format=", hash)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// CheckoutPaths restores the working tree (all paths) to the state at hash,
// used to source a rollback before re-committing.
func (s *Service) CheckoutPaths(ctx context.Context, dir, hash string) error {
	if err := validateRef(hash); err != nil {
		return errors.New("invalid commit ref")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.runCmd(ctx, dir, s.timeout, "checkout", hash, "--", ".")
	return err
}
