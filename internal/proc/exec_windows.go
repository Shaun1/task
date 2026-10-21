//go:build windows

// Package proc hands the current terminal over to another program.
package proc

import (
	"os"
	"os/exec"
)

// Exec runs bin as a child in dir with inherited stdio (syscall.Exec is
// unavailable on Windows).
func Exec(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
