package internal

import (
	"golang.org/x/term"
)

// IsInteractive returns true if the given file descriptor is a TTY.
// This is used to determine if interactive prompts should be shown.
func IsInteractive(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}
