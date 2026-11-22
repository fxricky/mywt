//go:build !unix

package picker

import (
	"errors"
	"fmt"
	"os"
)

// ErrCancelled is returned when the user leaves the selector without choosing.
var ErrCancelled = errors.New("cancelled")

// ErrNoSessions is returned when there is nothing to select.
var ErrNoSessions = errors.New("no sessions")

// Run reports that the interactive selector is only implemented for Unix
// terminals. mywt's supported environments are macOS and Linux.
func Run(in, out *os.File, items []Item) (int, error) {
	return -1, fmt.Errorf("interactive session selector requires a Unix terminal")
}
