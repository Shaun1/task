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
	ran      []string // "name dir"
	removed  []string
	runErr   error
	warnings []string
	attached []string
	shells   []string // "name dir"
}

func (f *fakeSandbox) Attach(name string) error { f.attached = append(f.attached, name); return nil }

func (f *fakeSandbox) Shell(name, dir string) error {
	f.shells = append(f.shells, name+" "+dir)
	return nil
}

type fakeHost struct {
	shells []string
	ides   []string // "command dir"
	found  []string
}

func (h *fakeHost) IDECandidates() []string { return h.found }

func (h *fakeHost) Shell(dir string) error { h.shells = append(h.shells, dir); return nil }
func (h *fakeHost) OpenIDE(command, dir string) error {
	h.ides = append(h.ides, command+" "+dir)
	return nil
}

func (f *fakeSandbox) Name(dir string) string   { return "claude-" + filepath.Base(dir) }
func (f *fakeSandbox) List() ([]string, error)  { return f.existing, nil }
func (f *fakeSandbox) Warnings(string) []string { return f.warnings }
func (f *fakeSandbox) Run(name, dir string) error {
	f.ran = append(f.ran, name+" "+dir)
	return f.runErr
}
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
	return Env{Cfg: config.Config{Home: t.TempDir()}, Sandbox: fs, Host: &fakeHost{}, Out: &buf, Cwd: work}, fs, origin
}

func TestStartCreatesCloneBranchStateAndLaunches(t *testing.T) {
	env, fs, origin := newEnv(t)
	testutil.Git(t, env.Cwd, "config", "user.email", "dev@example.com")
	if err := env.New("Fix login redirect bug"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "fix-login-redirect-bug")
	if want := "claude-" + filepath.Base(dest) + " " + dest; len(fs.ran) != 1 || fs.ran[0] != want {
		t.Fatalf("sandbox ran %v, want [%s]", fs.ran, want)
	}
	if got := testutil.Git(t, dest, "branch", "--show-current"); got != "fix-login-redirect-bug" {
		t.Fatalf("branch = %q", got)
	}
	if got := testutil.Git(t, dest, "config", "--local", "user.email"); got != "dev@example.com" {
		t.Fatalf("clone user.email = %q", got)
	}
	tk, err := env.store().Load("fix-login-redirect-bug")
	if err != nil {
		t.Fatal(err)
	}
	if tk.SandboxName != "claude-fix-login-redirect-bug" || tk.OriginURL != origin || tk.ClonePath != dest {
		t.Fatalf("bad state: %+v", tk)
	}
	if err := env.New("fix login redirect bug"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate new err = %v", err)
	}
}

func TestStartPrintsWarningsAndContinues(t *testing.T) {
	env, fs, _ := newEnv(t)
	fs.warnings = []string{"no keys"}
	if err := env.New("warn me"); err != nil {
		t.Fatal(err)
	}
	if out := env.Out.(*bytes.Buffer).String(); !strings.Contains(out, "warning: no keys\n") {
		t.Fatalf("output missing warning:\n%s", out)
	}
	if len(fs.ran) != 1 {
		t.Fatalf("sandbox should still launch, ran=%v", fs.ran)
	}
}

func TestStartFailsOutsideRepo(t *testing.T) {
	env, _, _ := newEnv(t)
	env.Cwd = t.TempDir()
	if err := env.New("anything"); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("err = %v", err)
	}
}

func TestStartRollsBackWhenLaunchFails(t *testing.T) {
	env, fs, _ := newEnv(t)
	fs.runErr = os.ErrPermission
	if err := env.New("will fail"); err == nil {
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
	if err := env.New("dupe"); err == nil || !strings.Contains(err.Error(), "sandbox") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveSafetyChecksAndForce(t *testing.T) {
	env, fs, _ := newEnv(t)
	if err := env.New("my task"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "my-task")

	// untracked file -> dirty -> refuse
	if err := os.WriteFile(filepath.Join(dest, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.Remove("my-task", false); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("expected uncommitted refusal, got %v", err)
	}

	// committed but unpushed -> refuse
	testutil.Git(t, dest, "add", ".")
	testutil.Git(t, dest, "commit", "-m", "wip")
	if err := env.Remove("my-task", false); err == nil || !strings.Contains(err.Error(), "unpushed") {
		t.Fatalf("expected unpushed refusal, got %v", err)
	}

	// pushed -> remove succeeds; ErrNotFound from sandbox is ignored
	testutil.Git(t, dest, "push", "-u", "origin", "my-task")
	if err := env.Remove("my-task", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("clone should be gone")
	}
	if _, err := env.store().Load("my-task"); err == nil {
		t.Fatal("state should be gone")
	}
	if len(fs.removed) != 1 || fs.removed[0] != "claude-my-task" {
		t.Fatalf("removed=%v", fs.removed)
	}
}

func TestRemoveForceDiscardsAndInfersFromCwd(t *testing.T) {
	env, _, _ := newEnv(t)
	if err := env.New("force me"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "force-me")
	if err := os.WriteFile(filepath.Join(dest, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.Cwd = dest // infer from cwd
	if err := env.Remove("", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("clone should be gone")
	}
}

func TestRemoveRefusesPathOutsideClonesDir(t *testing.T) {
	env, _, _ := newEnv(t)
	victim := t.TempDir()
	if err := env.store().Save(state.Task{Name: "evil", ClonePath: victim, SandboxName: "claude-evil"}); err != nil {
		t.Fatal(err)
	}
	if err := env.Remove("evil", true); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("victim dir must survive")
	}
}

func TestRemoveUnknownAndList(t *testing.T) {
	env, _, _ := newEnv(t)
	if err := env.Remove("nope", false); err == nil {
		t.Fatal("expected error")
	}
	if err := env.Remove("", false); err == nil {
		t.Fatal("expected error when not in a clone")
	}
	var buf bytes.Buffer
	env.Out = &buf
	if err := env.List(); err != nil || !strings.Contains(buf.String(), "No tasks") {
		t.Fatalf("list empty: %q %v", buf.String(), err)
	}
	if err := env.New("listed one"); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := env.List(); err != nil || !strings.Contains(buf.String(), "listed-one") {
		t.Fatalf("list: %q %v", buf.String(), err)
	}
}

func TestDo(t *testing.T) {
	env, fs, _ := newEnv(t)
	if err := env.New("open me"); err != nil {
		t.Fatal(err)
	}
	h := env.Host.(*fakeHost)
	dest := filepath.Join(env.Cfg.ClonesDir(), "origin", "open-me")

	// Claude: refuses while the sandbox is missing, attaches once it exists.
	if err := env.Do("open-me", TargetClaude); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("missing sandbox err = %v", err)
	}
	if err := env.Do("open-me", TargetSandboxShell); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("missing sandbox shell err = %v", err)
	}
	fs.existing = []string{"claude-open-me"}
	if err := env.Do("open-me", TargetClaude); err != nil || len(fs.attached) != 1 || fs.attached[0] != "claude-open-me" {
		t.Fatalf("attach: err=%v attached=%v", err, fs.attached)
	}

	// Sandbox shell: in the clone, inside the task's sandbox.
	if err := env.Do("open-me", TargetSandboxShell); err != nil || len(fs.shells) != 1 || fs.shells[0] != "claude-open-me "+dest {
		t.Fatalf("sandbox shell: err=%v shells=%v", err, fs.shells)
	}

	// Shell, with the task inferred from a directory inside the clone.
	env.Cwd = filepath.Join(dest, "sub")
	if err := os.Mkdir(env.Cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := env.Do("", TargetShell); err != nil || len(h.shells) != 1 || h.shells[0] != dest {
		t.Fatalf("shell: err=%v shells=%v", err, h.shells)
	}

	// IDE: non-interactive and unconfigured is an error.
	if err := env.Do("open-me", TargetIDE); err == nil || !strings.Contains(err.Error(), "no IDE configured") {
		t.Fatalf("unconfigured IDE err = %v", err)
	}
	env.Cfg.IDE = "goland"
	if err := env.Do("open-me", TargetIDE); err != nil || len(h.ides) != 1 || h.ides[0] != "goland "+dest {
		t.Fatalf("ide: err=%v ides=%v", err, h.ides)
	}
}

func TestDoIDEPromptsOnFirstUse(t *testing.T) {
	env, _, _ := newEnv(t)
	if err := env.New("ide me"); err != nil {
		t.Fatal(err)
	}
	h := env.Host.(*fakeHost)
	h.found = []string{"goland", "pycharm"}
	var gotDef string
	env.Prompt = func(_, def string) (string, error) { gotDef = def; return def, nil }

	if err := env.Do("ide-me", TargetIDE); err != nil {
		t.Fatal(err)
	}
	if gotDef != "goland" {
		t.Fatalf("default offered = %q, want goland", gotDef)
	}
	if len(h.ides) != 1 || !strings.HasPrefix(h.ides[0], "goland ") {
		t.Fatalf("ides = %v", h.ides)
	}
	saved, err := os.ReadFile(env.Cfg.File())
	if err != nil || !strings.Contains(string(saved), `"ide": "goland"`) {
		t.Fatalf("config.json = %s, %v", saved, err)
	}

	// An empty answer with nothing found is refused and nothing is saved.
	env2, _, _ := newEnv(t)
	if err := env2.New("ide me"); err != nil {
		t.Fatal(err)
	}
	env2.Prompt = func(string, string) (string, error) { return " ", nil }
	if err := env2.Do("ide-me", TargetIDE); err == nil || !strings.Contains(err.Error(), "no IDE given") {
		t.Fatalf("empty answer err = %v", err)
	}
	if _, err := os.Stat(env2.Cfg.File()); err == nil {
		t.Fatal("config.json should not be written")
	}
}
