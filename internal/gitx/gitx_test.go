package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"task/internal/testutil"
)

func TestRepoRootAndOrigin(t *testing.T) {
	origin := testutil.NewOrigin(t)
	work := testutil.NewWorkingRepo(t, origin)

	root, err := RepoRoot(filepath.Join(work))
	if err != nil {
		t.Fatal(err)
	}
	if root == "" {
		t.Fatal("empty root")
	}
	got, err := OriginURL(work)
	if err != nil || got != origin {
		t.Fatalf("OriginURL = %q, %v; want %q", got, err, origin)
	}
}

func TestNotRepoAndNoOrigin(t *testing.T) {
	testutil.HermeticGit(t)
	plain := t.TempDir()
	if _, err := RepoRoot(plain); !errors.Is(err, ErrNotRepo) {
		t.Errorf("RepoRoot err = %v, want ErrNotRepo", err)
	}
	testutil.Git(t, plain, "init", "-b", "main")
	if _, err := OriginURL(plain); !errors.Is(err, ErrNoOrigin) {
		t.Errorf("OriginURL err = %v, want ErrNoOrigin", err)
	}
}

func TestCloneBranchDirtyUnpushed(t *testing.T) {
	origin := testutil.NewOrigin(t)
	dest := filepath.Join(t.TempDir(), "clones", "repo", "my-task")

	if err := Clone(origin, "main", dest); err != nil {
		t.Fatal(err)
	}
	// Self-contained: .git must be a directory, not a pointer file.
	if fi, err := os.Stat(filepath.Join(dest, ".git")); err != nil || !fi.IsDir() {
		t.Fatalf(".git should be a directory: %v", err)
	}
	if err := CheckoutNewBranch(dest, "my-task"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Git(t, dest, "branch", "--show-current"); got != "my-task" {
		t.Fatalf("branch = %q", got)
	}

	if dirty, err := IsDirty(dest); err != nil || dirty {
		t.Fatalf("fresh clone dirty=%v err=%v", dirty, err)
	}
	if un, err := UnpushedCommits(dest); err != nil || len(un) != 0 {
		t.Fatalf("fresh clone unpushed=%v err=%v", un, err)
	}

	if err := os.WriteFile(filepath.Join(dest, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := IsDirty(dest); !dirty {
		t.Fatal("expected dirty after adding file")
	}

	testutil.Git(t, dest, "add", ".")
	testutil.Git(t, dest, "commit", "-m", "work")
	if dirty, _ := IsDirty(dest); dirty {
		t.Fatal("expected clean after commit")
	}
	if un, err := UnpushedCommits(dest); err != nil || len(un) != 1 {
		t.Fatalf("after commit unpushed=%v err=%v, want 1", un, err)
	}

	testutil.Git(t, dest, "push", "-u", "origin", "my-task")
	if un, err := UnpushedCommits(dest); err != nil || len(un) != 0 {
		t.Fatalf("after push unpushed=%v err=%v, want 0", un, err)
	}
}
