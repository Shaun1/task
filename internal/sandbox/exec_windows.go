//go:build windows

package sandbox

import (
	"os"
	"os/exec"
)

// execInteractive runs the command as a child with inherited stdio
// (syscall.Exec is unavailable on Windows).
func execInteractive(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
