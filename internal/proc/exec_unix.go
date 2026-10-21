//go:build !windows

// Package proc hands the current terminal over to another program.
package proc

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Exec replaces the current process with bin, started in dir ("" keeps the
// current directory), so the TTY and signals behave exactly as if the user
// had run it directly. It only returns on failure.
func Exec(dir, bin string, args ...string) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w", bin, err)
	}
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return err
		}
	}
	return syscall.Exec(path, append([]string{bin}, args...), os.Environ())
}
