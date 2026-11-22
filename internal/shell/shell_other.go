//go:build !unix

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func runInteractive(cmd *exec.Cmd, tty *os.File) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	return waitErr(cmd.Wait())
}

func waitErr(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("shell: %w", err)
	}
	if code := exitErr.ExitCode(); code > 0 {
		return &ExitError{Code: code}
	}
	return &ExitError{Code: 1}
}
