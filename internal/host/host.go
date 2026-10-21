// Package host starts programs on the user's machine (outside any sandbox).
package host

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"task/internal/proc"
)

// Host implements workflow.Host.
type Host struct{}

// Shell replaces the current process with the user's $SHELL in dir.
func (Host) Shell(dir string) error {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	return proc.Exec(dir, sh)
}

// OpenIDE runs command through the shell with dir as its last argument, so
// the command may carry its own arguments and quoting. JetBrains Toolbox's
// launcher scripts are found even if their directory isn't on PATH.
func (Host) OpenIDE(command, dir string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command+` "`+dir+`"`)
	} else {
		cmd = exec.Command("sh", "-c", command+` "$1"`, "sh", dir)
	}
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+string(os.PathListSeparator)+toolboxScripts())
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// knownIDEs are launcher commands offered when no IDE is configured, in
// order of preference.
var knownIDEs = []string{"goland", "idea", "pycharm", "webstorm", "rustrover", "clion", "rider", "code", "cursor", "zed"}

// IDECandidates lists known IDE launchers found on PATH or in Toolbox's
// scripts directory.
func (Host) IDECandidates() []string {
	dirs := append(filepath.SplitList(os.Getenv("PATH")), toolboxScripts())
	var found []string
	for _, name := range knownIDEs {
		for _, d := range dirs {
			if fi, err := os.Stat(filepath.Join(d, name)); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
				found = append(found, name)
				break
			}
		}
	}
	return found
}

// toolboxScripts is where JetBrains Toolbox puts IDE launchers (goland, ...).
func toolboxScripts() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "JetBrains", "Toolbox", "scripts")
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "JetBrains", "Toolbox", "scripts")
	default:
		return filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "scripts")
	}
}
