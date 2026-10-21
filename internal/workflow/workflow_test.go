package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"task/internal/config"
	"task/internal/sandbox"
	"task/internal/state"
	"task/internal/testutil"
)

type fakeSandbox struct {
	existing []string
	ran      []string
	stopped  []string
	removed  []string
	runErr   error
}

func (f *fakeSandbox) Name(dir string) string  { return "claude-" + filepath.Base(dir) }
func (f *fakeSandbox) List() ([]string, error) { return f.existing, nil }
func (f *fakeSandbox) Run(dir string) error    { f.ran = append(f.ran, dir); return f.runErr }
func (f *fakeSandbox) Stop(name string) error  { f.stopped = append(f.stopped, name); return nil }
func (f *fakeSandbox) Remove(name string) error {
	f.removed = append(f.removed, name)
	return sandbox.ErrNotFound
}

func newEnv(t *testing.T) (Env, *fakeSandbox, string) {
	t.Helper()
	origin := testutil.NewOrigin(t)
	work := testutil.NewWorkingRepo(t, origin)
	fs := &fakeSandbox{}
	var buf bytes.Buffer
	return Env{Cfg: config.Config{Home: t.TempDir()}, Sandbox: fs, Out: &buf, Cwd: work}, fs, origin
}

func TestStartCreatesCloneBranchStateAndLaunches(t *testing.T) {
	env, fs, origin := newEnv(t)
	if err := env.Start("Fix login redirect bug"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "fix-login-redirect-bug")
	if len(fs.ran) != 1 || fs.ran[0] != dest {
		t.Fatalf("sandbox ran %v, want [%s]", fs.ran, dest)
	}
	if got := testutil.Git(t, dest, "branch", "--show-current"); got != "fix-login-redirect-bug" {
		t.Fatalf("branch = %q", got)
	}
	tk, err := env.store().Load("fix-login-redirect-bug")
	if err != nil {
		t.Fatal(err)
	}
	if tk.SandboxName != "claude-fix-login-redirect-bug" || tk.OriginURL != origin || tk.ClonePath != dest {
		t.Fatalf("bad state: %+v", tk)
	}
	if err := env.Start("fix login redirect bug"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate start err = %v", err)
	}
}

func TestStartFailsOutsideRepo(t *testing.T) {
	env, _, _ := newEnv(t)
	env.Cwd = t.TempDir()
	if err := env.Start("anything"); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("err = %v", err)
	}
}

func TestStartRollsBackWhenLaunchFails(t *testing.T) {
	env, fs, _ := newEnv(t)
	fs.runErr = os.ErrPermission
	if err := env.Start("will fail"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(env.Cfg.ClonesDir(), "origin", "will-fail")); !os.IsNotExist(err) {
		t.Fatalf("clone should be removed, stat err = %v", err)
	}
	if _, err := env.store().Load("will-fail"); err == nil {
		t.Fatal("state should be removed")
	}
}

func TestStartRefusesExistingSandbox(t *testing.T) {
	env, fs, _ := newEnv(t)
	fs.existing = []string{"claude-dupe"}
	if err := env.Start("dupe"); err == nil || !strings.Contains(err.Error(), "sandbox") {
		t.Fatalf("err = %v", err)
	}
}

func TestCleanSafetyChecksAndForce(t *testing.T) {
	env, fs, _ := newEnv(t)
	if err := env.Start("my task"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "my-task")

	// untracked file -> dirty -> refuse
	if err := os.WriteFile(filepath.Join(dest, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.Clean("my-task", false); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("expected uncommitted refusal, got %v", err)
	}

	// committed but unpushed -> refuse
	testutil.Git(t, dest, "add", ".")
	testutil.Git(t, dest, "commit", "-m", "wip")
	if err := env.Clean("my-task", false); err == nil || !strings.Contains(err.Error(), "unpushed") {
		t.Fatalf("expected unpushed refusal, got %v", err)
	}

	// pushed -> clean succeeds; ErrNotFound from sandbox is ignored
	testutil.Git(t, dest, "push", "-u", "origin", "my-task")
	if err := env.Clean("my-task", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("clone should be gone")
	}
	if _, err := env.store().Load("my-task"); err == nil {
		t.Fatal("state should be gone")
	}
	if len(fs.stopped) != 1 || fs.stopped[0] != "claude-my-task" || len(fs.removed) != 1 {
		t.Fatalf("stopped=%v removed=%v", fs.stopped, fs.removed)
	}
}

func TestCleanForceDiscardsAndInfersFromCwd(t *testing.T) {
	env, _, _ := newEnv(t)
	if err := env.Start("force me"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "force-me")
	if err := os.WriteFile(filepath.Join(dest, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.Cwd = dest // infer from cwd
	if err := env.Clean("", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("clone should be gone")
	}
}

func TestCleanRefusesPathOutsideClonesDir(t *testing.T) {
	env, _, _ := newEnv(t)
	victim := t.TempDir()
	if err := env.store().Save(state.Task{Name: "evil", ClonePath: victim, SandboxName: "claude-evil"}); err != nil {
		t.Fatal(err)
	}
	if err := env.Clean("evil", true); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("victim dir must survive")
	}
}

func TestCleanUnknownAndList(t *testing.T) {
	env, _, _ := newEnv(t)
	if err := env.Clean("nope", false); err == nil {
		t.Fatal("expected error")
	}
	if err := env.Clean("", false); err == nil {
		t.Fatal("expected error when not in a clone")
	}
	var buf bytes.Buffer
	env.Out = &buf
	if err := env.List(); err != nil || !strings.Contains(buf.String(), "No tasks") {
		t.Fatalf("list empty: %q %v", buf.String(), err)
	}
	if err := env.Start("listed one"); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := env.List(); err != nil || !strings.Contains(buf.String(), "listed-one") {
		t.Fatalf("list: %q %v", buf.String(), err)
	}
}
