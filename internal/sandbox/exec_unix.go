//go:build !windows

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// execInteractive replaces the current process so the TTY and signals behave
// exactly as if the user had run sbx directly. It only returns on failure.
func execInteractive(bin string, args []string) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w", bin, err)
	}
	return syscall.Exec(path, append([]string{bin}, args...), os.Environ())
}
