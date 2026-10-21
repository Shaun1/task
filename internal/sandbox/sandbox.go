// Package sandbox is the only package that knows about the sbx CLI.
package sandbox

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Agent is the sbx agent launched in each sandbox.
const Agent = "claude"

// ErrNotFound is returned by Stop/Remove when the sandbox doesn't exist.
var ErrNotFound = errors.New("sandbox not found")

// Sandbox abstracts the sandbox runtime so workflows can be tested with a fake.
type Sandbox interface {
	// Name predicts the sandbox name sbx assigns for a workspace directory.
	Name(dir string) string
	// List returns the names of existing sandboxes.
	List() ([]string, error)
	// Run creates/attaches a sandbox for dir in the current terminal.
	// On unix it replaces the current process and only returns on failure.
	Run(dir string) error
	Stop(name string) error
	Remove(name string) error
}

// SBX implements Sandbox on top of the standalone `sbx` CLI.
type SBX struct {
	Bin string
}

// New returns an SBX using `sbx` from PATH.
func New() *SBX { return &SBX{Bin: "sbx"} }

// Name: spike showed sbx names sandboxes "<agent>-<workspace dir name>".
func (s *SBX) Name(dir string) string { return Agent + "-" + filepath.Base(dir) }

func (s *SBX) List() ([]string, error) {
	out, err := exec.Command(s.Bin, "ls").Output()
	if err != nil {
		return nil, fmt.Errorf("%s ls: %w", s.Bin, err)
	}
	return ParseList(string(out)), nil
}

func (s *SBX) Run(dir string) error {
	return execInteractive(s.Bin, []string{"run", Agent, dir})
}

func (s *SBX) Stop(name string) error   { return s.simple("stop", name) }
func (s *SBX) Remove(name string) error { return s.simple("rm", name) }

func (s *SBX) simple(verb, name string) error {
	cmd := exec.Command(s.Bin, verb, name)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(buf.String())
		low := strings.ToLower(msg)
		if strings.Contains(low, "not found") || strings.Contains(low, "no such") || strings.Contains(low, "does not exist") {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return fmt.Errorf("%s %s %s: %w: %s", s.Bin, verb, name, err, msg)
	}
	return nil
}

// ParseList extracts sandbox names (first column) from `sbx ls` output.
//
// NOTE: the exact `sbx ls` format hasn't been verified. This assumes a
// whitespace-separated table whose first column is the name, with an optional
// header row. Adjust here if the real format differs.
func ParseList(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		first := strings.ToUpper(fields[0])
		if first == "NAME" || first == "NAMES" || first == "SANDBOX" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "no ") {
			continue // e.g. "No sandboxes found"
		}
		names = append(names, fields[0])
	}
	return names
}
