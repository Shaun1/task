// Command task opens an isolated Claude Code sandbox for a task: a fresh clone
// of origin/main on a new branch, launched via sbx; and removes it afterwards.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"task/internal/config"
	"task/internal/host"
	"task/internal/sandbox"
	"task/internal/state"
	"task/internal/workflow"
)

func newEnv() (workflow.Env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return workflow.Env{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return workflow.Env{}, err
	}
	env := workflow.Env{Cfg: cfg, Sandbox: sandbox.New(), Host: host.Host{}, Out: os.Stdout, Cwd: cwd}
	// A char-device check isn't enough: /dev/null is one too.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		env.Prompt = prompt
	}
	return env, nil
}

// prompt reads one line from stdin; an empty answer means def.
func prompt(question, def string) (string, error) {
	if def != "" {
		fmt.Printf("%s [%s]: ", question, def)
	} else {
		fmt.Printf("%s: ", question)
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read answer: %w", err)
	}
	if line = strings.TrimSpace(line); line != "" {
		return line, nil
	}
	return def, nil
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "task",
		Short:         "Run a task in an isolated Claude Code sandbox",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildVersion(),
	}
	root.AddCommand(newStartCmd(), newOpenCmd(), newLsCmd(), newRmCmd())
	return root
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <description>",
		Short: "Clone origin/main, branch from the description, open Claude Code in a sandbox",
		Example: `  task new fix login redirect bug
  task new "fix login redirect bug"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := newEnv()
			if err != nil {
				return err
			}
			return env.New(strings.Join(args, " "))
		},
	}
}

func newOpenCmd() *cobra.Command {
	var shell, sbx, ide bool
	cmd := &cobra.Command{
		Use:   "open [name] [--shell | --sbx | --ide]",
		Short: "Open a new Claude session, shell or IDE on a task (name inferred from cwd if omitted)",
		Long: `Attach a new process to an existing task.

By default this starts another Claude Code session in the task's sandbox.
--shell opens your $SHELL on this machine in the task's clone instead,
--sbx opens a bash shell inside the task's sandbox, and --ide opens the clone in the IDE set in ~/.task/config.json, e.g.:

  {"ide": "goland"}

The IDE value is a shell command; the clone path is appended to it.`,
		Example: `  task open fix-login-redirect-bug
  task open --shell
  task open --sbx
  task open --ide`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := newEnv()
			if err != nil {
				return err
			}
			var name string
			if len(args) == 1 {
				name = args[0]
			}
			target := workflow.TargetClaude
			switch {
			case shell:
				target = workflow.TargetShell
			case sbx:
				target = workflow.TargetSandboxShell
			case ide:
				target = workflow.TargetIDE
			}
			return env.Open(name, target)
		},
		ValidArgsFunction: completeTaskNames,
	}
	cmd.Flags().BoolVar(&shell, "shell", false, "open your shell on this machine, in the task's clone")
	cmd.Flags().BoolVar(&sbx, "sbx", false, "open a shell inside the task's sandbox, in the clone")
	cmd.Flags().BoolVar(&ide, "ide", false, "open the task's clone in your configured IDE")
	cmd.MarkFlagsMutuallyExclusive("shell", "sbx", "ide")
	return cmd
}

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := newEnv()
			if err != nil {
				return err
			}
			return env.List()
		},
	}
}

func newRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm [name]",
		Short: "Remove a task's sandbox and clone (name inferred from cwd if omitted)",
		Long: `Remove a task's sandbox and clone.

Refuses if the clone has uncommitted changes or commits not on any origin
branch, since the clone is the only copy. Push your branch first, or pass
--force to discard the work.`,
		Example: "  task rm fix-login-redirect-bug",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := newEnv()
			if err != nil {
				return err
			}
			var name string
			if len(args) == 1 {
				name = args[0]
			}
			return env.Remove(name, force)
		},
		ValidArgsFunction: completeTaskNames,
	}
	cmd.Flags().BoolVar(&force, "force", false, "discard uncommitted/unpushed work")
	return cmd
}

// completeTaskNames offers existing task names for shell completion.
func completeTaskNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	tasks, err := state.Store{Dir: cfg.StateDir()}.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(tasks))
	for _, t := range tasks {
		names = append(names, t.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
