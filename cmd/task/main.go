// Command task opens an isolated Claude Code sandbox for a task: a fresh clone
// of origin/main on a new branch, launched via sbx; and cleans it up afterwards.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"task/internal/config"
	"task/internal/sandbox"
	"task/internal/workflow"
)

const usage = `task - run a task in an isolated Claude Code sandbox

Usage:
  task start <description>        clone origin/main, create a branch from the
                                  description, open Claude Code in a sandbox
  task ls                         list tasks
  task clean [name] [--force]     remove a task's sandbox and clone
  task help                       show this help

Examples:
  task start fix login redirect bug
  task clean fix-login-redirect-bug
`

func newEnv() (workflow.Env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return workflow.Env{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return workflow.Env{}, err
	}
	return workflow.Env{Cfg: cfg, Sandbox: sandbox.New(), Out: os.Stdout, Cwd: cwd}, nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("missing command")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil

	case "start":
		if len(rest) == 0 {
			return fmt.Errorf("usage: task start <description>")
		}
		env, err := newEnv()
		if err != nil {
			return err
		}
		return env.Start(strings.Join(rest, " "))

	case "ls":
		if len(rest) != 0 {
			return fmt.Errorf("usage: task ls")
		}
		env, err := newEnv()
		if err != nil {
			return err
		}
		return env.List()

	case "clean":
		fs := flag.NewFlagSet("clean", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		force := fs.Bool("force", false, "discard uncommitted/unpushed work")
		// Allow the name and --force in either order.
		var name string
		for len(rest) > 0 {
			if err := fs.Parse(rest); err != nil {
				return fmt.Errorf("usage: task clean [name] [--force]")
			}
			rest = fs.Args()
			if len(rest) > 0 {
				if name != "" {
					return fmt.Errorf("usage: task clean [name] [--force]")
				}
				name, rest = rest[0], rest[1:]
			}
		}
		env, err := newEnv()
		if err != nil {
			return err
		}
		return env.Clean(name, *force)

	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}
