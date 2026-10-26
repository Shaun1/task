// Package sandbox is the only package that knows about the sbx CLI.
package sandbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"task/internal/proc"
)

// Agent is the sbx agent launched in each sandbox.
const Agent = "claude"

// ErrNotFound is returned by Stop/Remove when the sandbox doesn't exist.
var ErrNotFound = errors.New("sandbox not found")

// Sandbox abstracts the sandbox runtime so workflows can be tested with a fake.
type Sandbox interface {
	// Name returns the sandbox name to use for a workspace directory.
	Name(dir string) string
	// List returns the names of existing sandboxes.
	List() ([]string, error)
	// Run creates/attaches sandbox name for dir in the current terminal.
	// On unix it replaces the current process and only returns on failure.
	Run(name, dir string) error
	// Attach starts a new agent session in existing sandbox name, in the
	// current terminal. On unix it only returns on failure.
	Attach(name string) error
	// Shell opens an interactive shell in existing sandbox name, in dir, in
	// the current terminal. On unix it only returns on failure.
	Shell(name, dir string) error
	// Remove stops and deletes a sandbox without prompting.
	Remove(name string) error
	// Warnings reports host setup missing for working on originURL inside
	// a sandbox. Checks that can't run are skipped, not reported.
	Warnings(originURL string) []string
}

// SBX implements Sandbox on top of the standalone `sbx` CLI.
type SBX struct {
	Bin string
}

// New returns an SBX using `sbx` from PATH.
func New() *SBX { return &SBX{Bin: "sbx"} }

// Name mirrors sbx's default "<agent>-<workspace dir name>"; Run passes it
// explicitly via --name so it never has to be guessed.
func (s *SBX) Name(dir string) string { return Agent + "-" + filepath.Base(dir) }

func (s *SBX) List() ([]string, error) {
	out, err := exec.Command(s.Bin, "ls", "-q").Output()
	if err != nil {
		return nil, fmt.Errorf("%s ls: %w", s.Bin, err)
	}
	return ParseList(string(out)), nil
}

// githubSSH is opened per sandbox: sbx blocks port 22 by default. Auth goes
// through the host's SSH agent, which sbx forwards (ssh.agentForwardingEnabled),
// so no private key enters the sandbox.
const githubSSH = "github.com:22"

// sshEnv lets git accept GitHub's host key on first use; the sandbox starts
// with no known_hosts and git can't answer the prompt non-interactively.
const sshEnv = "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new"

// Run creates the sandbox first because sbx only accepts a sandbox-scoped
// policy rule for a sandbox that already exists, then attaches to it.
func (s *SBX) Run(name, dir string) error {
	if err := s.run("create", "--name", name, "-e", sshEnv, Agent, dir); err != nil {
		return err
	}
	if err := s.run("policy", "allow", "network", "--sandbox", name, githubSSH); err != nil {
		_ = s.Remove(name)
		return err
	}
	return s.Attach(name)
}

// Attach starts another agent session in an existing sandbox. Each
// `sbx run --name` gets its own agent process.
func (s *SBX) Attach(name string) error {
	return proc.Exec("", s.Bin, "run", "--name", name)
}

// Shell runs bash in the sandbox via `sbx exec`. Env set with `create -e`
// only reaches agent sessions, so the SSH setting is passed again.
func (s *SBX) Shell(name, dir string) error {
	return proc.Exec("", s.Bin, "exec", "-it", "-e", sshEnv, "-w", dir, name, "bash")
}

// Remove uses --force because `sbx rm` otherwise asks for confirmation. It
// also stops the sandbox, so no separate `sbx stop` is needed.
func (s *SBX) Remove(name string) error {
	err := s.run("rm", "--force", name)
	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "not found") || strings.Contains(low, "no such") || strings.Contains(low, "does not exist") {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
	}
	return err
}

// run runs sbx non-interactively, folding its output into any error.
func (s *SBX) run(args ...string) error {
	cmd := exec.Command(s.Bin, args...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", s.Bin, strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
	}
	return nil
}

func (s *SBX) Warnings(originURL string) []string {
	if !strings.Contains(originURL, "github.com") {
		return nil
	}
	var warnings []string
	if isSSHURL(originURL) {
		if msg := sshAgentProblem(); msg != "" {
			warnings = append(warnings, msg)
		}
	}
	if ok, err := s.hasGitHubSecret(); err == nil && !ok {
		warnings = append(warnings, `no "github" sbx secret, so gh (e.g. opening a PR) won't authenticate in the sandbox; set it with: sbx secret set github --command 'gh auth token'`)
	}
	return warnings
}

// isSSHURL reports whether a git remote URL uses SSH (ssh:// or scp-style).
func isSSHURL(u string) bool {
	return strings.HasPrefix(u, "ssh://") || !strings.Contains(u, "://")
}

// sshAgentProblem describes why the host SSH agent can't sign for the
// sandbox, or returns "" if it has keys or ssh-add isn't available.
func sshAgentProblem() string {
	err := exec.Command("ssh-add", "-l").Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return "" // keys loaded, or ssh-add missing
	}
	switch exitErr.ExitCode() {
	case 1:
		return "your SSH agent has no keys loaded, so git push over SSH will fail in the sandbox; load one with: ssh-add"
	case 2:
		return "no SSH agent is reachable (SSH_AUTH_SOCK), so git push over SSH will fail in the sandbox"
	}
	return ""
}

func (s *SBX) hasGitHubSecret() (bool, error) {
	out, err := exec.Command(s.Bin, "secret", "ls", "--global", "--service", "github", "--json").Output()
	if err != nil {
		return false, err
	}
	return parseHasSecret(out, "github")
}

// parseHasSecret reports whether `sbx secret ls --json` output lists a
// global secret for service.
func parseHasSecret(out []byte, service string) (bool, error) {
	var res struct {
		Secrets []struct {
			Scope string `json:"scope"`
			Name  string `json:"name"`
		} `json:"secrets"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return false, err
	}
	for _, sec := range res.Secrets {
		if sec.Name == service && sec.Scope == "global" {
			return true, nil
		}
	}
	return false, nil
}

// ParseList extracts sandbox names from `sbx ls -q` output (one per line).
// The plain table can't be parsed safely: sbx appends an update banner to it.
func ParseList(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}
