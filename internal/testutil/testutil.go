// Package testutil provides hermetic git helpers for tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// HermeticGit isolates git from the developer's global/system config.
func HermeticGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

// Git runs git in dir and fails the test on error.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// NewOrigin creates a bare repo with one commit on main and returns its path.
func NewOrigin(t *testing.T) string {
	t.Helper()
	HermeticGit(t)
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	Git(t, base, "init", "--bare", "-b", "main", origin)
	seed := filepath.Join(base, "seed")
	Git(t, base, "init", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Git(t, seed, "add", ".")
	Git(t, seed, "commit", "-m", "init")
	Git(t, seed, "remote", "add", "origin", origin)
	Git(t, seed, "push", "origin", "main")
	return origin
}

// NewWorkingRepo clones origin into a fresh temp dir (stand-in for "the repo you run task from").
func NewWorkingRepo(t *testing.T, origin string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "work")
	Git(t, filepath.Dir(dir), "clone", origin, dir)
	return dir
}
