// Package gitx wraps the git binary for the handful of operations task needs.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotRepo is returned when the directory is not inside a git repository.
var ErrNotRepo = errors.New("not a git repository")

// ErrNoOrigin is returned when the repository has no "origin" remote.
var ErrNoOrigin = errors.New("repository has no 'origin' remote")

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RepoRoot returns the top-level directory of the git repo containing dir.
func RepoRoot(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ErrNotRepo
	}
	return out, nil
}

// OriginURL returns the URL of the "origin" remote.
func OriginURL(dir string) (string, error) {
	out, err := run(dir, "remote", "get-url", "origin")
	if err != nil || out == "" {
		return "", ErrNoOrigin
	}
	return out, nil
}

// Clone makes a fully self-contained single-branch clone of url at dest.
// It deliberately avoids --shared/--reference: those leave pointers back to the
// source repo that don't resolve inside a sandbox.
func Clone(url, branch, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	_, err := run("", "clone", "--branch", branch, "--single-branch", url, dest)
	return err
}

// CheckoutNewBranch creates and checks out a new branch at the current HEAD.
func CheckoutNewBranch(dir, name string) error {
	_, err := run(dir, "checkout", "-b", name)
	return err
}

// IsDirty reports whether the working tree has uncommitted or untracked changes.
func IsDirty(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// UnpushedCommits lists commits on any local branch that are not reachable from
// any branch on origin. It asks the remote directly (ls-remote) rather than
// trusting remote-tracking refs, because a single-branch clone does not update
// tracking refs for branches pushed later.
func UnpushedCommits(dir string) ([]string, error) {
	out, err := run(dir, "ls-remote", "--heads", "origin")
	if err != nil {
		return nil, fmt.Errorf("could not query origin: %w", err)
	}
	args := []string{"rev-list", "--oneline", "--branches", "--not", "--remotes"}
	for _, line := range strings.Split(out, "\n") {
		sha, _, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		// Only pass commits we actually have locally; others can't be ancestors of our work.
		if _, err := run(dir, "cat-file", "-e", sha+"^{commit}"); err == nil {
			args = append(args, sha)
		}
	}
	res, err := run(dir, args...)
	if err != nil {
		return nil, err
	}
	if res == "" {
		return nil, nil
	}
	return strings.Split(res, "\n"), nil
}
