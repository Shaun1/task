// Package workflow implements the new / open / ls / rm commands on top of
// gitx, sandbox and state.
package workflow

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"task/internal/config"
	"task/internal/gitx"
	"task/internal/naming"
	"task/internal/sandbox"
	"task/internal/state"
)

// BaseBranch is always main (no --base flag in v1).
const BaseBranch = "main"

// Env carries dependencies so workflows are testable.
type Env struct {
	Cfg     config.Config
	Sandbox sandbox.Sandbox
	Host    Host
	Out     io.Writer
	Cwd     string
	// Prompt asks the user a question, showing def as the default answer.
	// Nil when not running interactively.
	Prompt func(question, def string) (string, error)
}

// Host starts programs on the user's machine.
type Host interface {
	// Shell replaces the current process with the user's shell in dir.
	Shell(dir string) error
	// OpenIDE runs command with dir appended.
	OpenIDE(command, dir string) error
	// IDECandidates lists IDE launcher commands installed on the host.
	IDECandidates() []string
}

// Target is what `task open` attaches to a task.
type Target int

const (
	TargetClaude Target = iota // new agent session in the task's sandbox
	TargetShell                // shell on the host, in the clone
	TargetIDE                  // configured IDE on the host, on the clone
)

func (e Env) store() state.Store { return state.Store{Dir: e.Cfg.StateDir()} }

// New clones the current repo's origin/main into a fresh directory, creates a
// branch named after the description, and opens it in a Claude Code sandbox.
func (e Env) New(desc string) error {
	root, err := gitx.RepoRoot(e.Cwd)
	if err != nil {
		return err
	}
	origin, err := gitx.OriginURL(root)
	if err != nil {
		return err
	}
	slug, err := naming.Slugify(desc)
	if err != nil {
		return err
	}
	repo := naming.RepoName(origin)
	dest := filepath.Join(e.Cfg.ClonesDir(), repo, slug)
	sbName := e.Sandbox.Name(dest)
	store := e.store()

	if _, err := store.Load(slug); err == nil {
		return fmt.Errorf("task %q already exists (finish it with: task rm %s)", slug, slug)
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	existing, err := e.Sandbox.List()
	if err != nil {
		return err
	}
	for _, n := range existing {
		if n == sbName {
			return fmt.Errorf("sandbox %q already exists", sbName)
		}
	}

	for _, w := range e.Sandbox.Warnings(origin) {
		fmt.Fprintln(e.Out, "warning:", w)
	}
	fmt.Fprintf(e.Out, "Cloning %s (%s) into %s\n", origin, BaseBranch, dest)
	if err := gitx.Clone(origin, BaseBranch, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	if err := gitx.CheckoutNewBranch(dest, slug); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	if err := gitx.CopyIdentity(root, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	rollback := func() {
		_ = store.Delete(slug)
		_ = os.RemoveAll(dest)
	}
	if err := store.Save(state.Task{
		Name: slug, Repo: repo, RepoRoot: root, OriginURL: origin,
		ClonePath: dest, Branch: slug, SandboxName: sbName, CreatedAt: time.Now().UTC(),
	}); err != nil {
		rollback()
		return err
	}

	fmt.Fprintf(e.Out, "Launching sandbox %s on branch %s\n", sbName, slug)
	if err := e.Sandbox.Run(sbName, dest); err != nil {
		rollback()
		return fmt.Errorf("launch sandbox: %w", err)
	}
	return nil
}

// Remove tears down a task's sandbox and clone. With name == "" it infers the
// task from the current directory. Without force it refuses if the clone has
// uncommitted or unpushed work, since the clone is the only copy.
func (e Env) Remove(name string, force bool) error {
	store := e.store()
	t, err := e.resolve(store, name)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(t.ClonePath); statErr == nil && !force {
		if problems := checkWork(t.ClonePath); len(problems) > 0 {
			return fmt.Errorf("refusing to remove %q; unsaved work in %s:\n  - %s\npush your branch (or re-run with --force to discard)",
				t.Name, t.ClonePath, strings.Join(problems, "\n  - "))
		}
	}

	if err := e.Sandbox.Remove(t.SandboxName); err != nil && !errors.Is(err, sandbox.ErrNotFound) {
		return fmt.Errorf("remove sandbox %s: %w", t.SandboxName, err)
	}
	fmt.Fprintf(e.Out, "Removed sandbox %s\n", t.SandboxName)

	if !within(e.Cfg.ClonesDir(), t.ClonePath) {
		return fmt.Errorf("refusing to delete %s: outside %s", t.ClonePath, e.Cfg.ClonesDir())
	}
	if err := os.RemoveAll(t.ClonePath); err != nil {
		return err
	}
	fmt.Fprintf(e.Out, "Removed %s\n", t.ClonePath)
	return store.Delete(t.Name)
}

// Open attaches a new process to an existing task. With name == "" it infers
// the task from the current directory.
func (e Env) Open(name string, target Target) error {
	t, err := e.resolve(e.store(), name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(t.ClonePath); err != nil {
		return fmt.Errorf("clone for %q is missing (%s); remove the task with: task rm %s", t.Name, t.ClonePath, t.Name)
	}
	switch target {
	case TargetShell:
		fmt.Fprintf(e.Out, "Opening a shell in %s (exit to return)\n", t.ClonePath)
		return e.Host.Shell(t.ClonePath)
	case TargetIDE:
		if e.Cfg.IDE == "" {
			if e.Cfg, err = e.configureIDE(); err != nil {
				return err
			}
		}
		fmt.Fprintf(e.Out, "Opening %s in %s\n", t.ClonePath, e.Cfg.IDE)
		return e.Host.OpenIDE(e.Cfg.IDE, t.ClonePath)
	default:
		// sbx run --name on a missing sandbox would create a new one on the
		// current directory, so check first.
		names, err := e.Sandbox.List()
		if err != nil {
			return err
		}
		for _, n := range names {
			if n == t.SandboxName {
				fmt.Fprintf(e.Out, "Starting a new Claude session in sandbox %s\n", t.SandboxName)
				return e.Sandbox.Attach(t.SandboxName)
			}
		}
		return fmt.Errorf("sandbox %s no longer exists; remove the task with: task rm %s", t.SandboxName, t.Name)
	}
}

// configureIDE asks for the IDE command on first use and saves it.
func (e Env) configureIDE() (config.Config, error) {
	cfg := e.Cfg
	if e.Prompt == nil {
		return cfg, fmt.Errorf(`no IDE configured; set one in %s, e.g. {"ide": "goland"}`, e.Cfg.File())
	}
	found := e.Host.IDECandidates()
	def := ""
	if len(found) > 0 {
		def = found[0]
		fmt.Fprintf(e.Out, "No IDE configured. Found: %s\n", strings.Join(found, ", "))
	} else {
		fmt.Fprintln(e.Out, "No IDE configured, and no known IDE launcher was found.")
	}
	ide, err := e.Prompt("IDE command (run with the clone path appended)", def)
	if err != nil {
		return cfg, err
	}
	if ide = strings.TrimSpace(ide); ide == "" {
		return cfg, errors.New("no IDE given")
	}
	cfg.IDE = ide
	if err := cfg.Save(); err != nil {
		return cfg, err
	}
	fmt.Fprintf(e.Out, "Saved to %s\n", cfg.File())
	return cfg, nil
}

// List prints all tasks.
func (e Env) List() error {
	tasks, err := e.store().List()
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		fmt.Fprintln(e.Out, "No tasks.")
		return nil
	}
	live := map[string]bool{}
	names, listErr := e.Sandbox.List()
	for _, n := range names {
		live[n] = true
	}
	w := tabwriter.NewWriter(e.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tREPO\tSANDBOX\tCREATED\tPATH")
	for _, t := range tasks {
		status := "missing"
		switch {
		case listErr != nil:
			status = "unknown"
		case live[t.SandboxName]:
			status = "present"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.Name, t.Repo, status, t.CreatedAt.Local().Format("2006-01-02 15:04"), t.ClonePath)
	}
	return w.Flush()
}

func (e Env) resolve(store state.Store, name string) (state.Task, error) {
	if name != "" {
		return store.Load(name)
	}
	tasks, err := store.List()
	if err != nil {
		return state.Task{}, err
	}
	for _, t := range tasks {
		if within(t.ClonePath, e.Cwd) {
			return t, nil
		}
	}
	return state.Task{}, errors.New("not inside a task clone; specify a task name (see: task ls)")
}

func checkWork(dir string) []string {
	var problems []string
	if dirty, err := gitx.IsDirty(dir); err != nil {
		problems = append(problems, "could not check working tree: "+err.Error())
	} else if dirty {
		problems = append(problems, "uncommitted changes")
	}
	if un, err := gitx.UnpushedCommits(dir); err != nil {
		problems = append(problems, "could not verify pushed state: "+err.Error())
	} else if len(un) > 0 {
		problems = append(problems, fmt.Sprintf("%d unpushed commit(s): %s", len(un), strings.Join(un, "; ")))
	}
	return problems
}

// within reports whether path is dir itself or inside it (symlinks resolved best-effort).
func within(dir, path string) bool {
	dir, path = resolveLinks(dir), resolveLinks(path)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func resolveLinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
