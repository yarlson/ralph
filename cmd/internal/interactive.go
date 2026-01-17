package internal

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

// IsInteractive returns true if the given file descriptor is a TTY.
// This is used to determine if interactive prompts should be shown.
func IsInteractive(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// UndoConfirmationInfo contains the information to display in the undo confirmation prompt.
type UndoConfirmationInfo struct {
	// IterationID is the ID of the iteration being undone.
	IterationID string
	// CommitToResetTo is the commit hash to reset to.
	CommitToResetTo string
	// TaskToReopen is the task ID that will be reopened (empty if none).
	TaskToReopen string
	// FilesToRevert is the list of files that will be reverted.
	FilesToRevert []string
	// HasUncommittedChanges indicates if there are uncommitted changes that will be lost.
	HasUncommittedChanges bool
}

// ConfirmUndo displays a confirmation prompt for the undo operation and reads the user's response.
// It shows the commit to reset to, task to reopen, files to revert, and warns about uncommitted changes.
// Returns true if the user confirms, false otherwise.
func ConfirmUndo(w io.Writer, r io.Reader, info UndoConfirmationInfo) (bool, error) {
	// Show what will happen
	_, _ = fmt.Fprintf(w, "Undo iteration %s:\n\n", info.IterationID)

	// Show commit to reset to (short hash)
	shortHash := info.CommitToResetTo
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	_, _ = fmt.Fprintf(w, "  Commit to reset to: %s\n", shortHash)

	// Show task to reopen (if any)
	if info.TaskToReopen != "" {
		_, _ = fmt.Fprintf(w, "  Task to reopen: %s\n", info.TaskToReopen)
	}

	// Show files to revert
	if len(info.FilesToRevert) > 0 {
		_, _ = fmt.Fprintf(w, "  Files to revert:\n")
		for _, file := range info.FilesToRevert {
			_, _ = fmt.Fprintf(w, "    - %s\n", file)
		}
	}
	_, _ = fmt.Fprintln(w)

	// Warn about uncommitted changes
	if info.HasUncommittedChanges {
		_, _ = fmt.Fprintln(w, "WARNING: You have uncommitted changes that will be lost!")
		_, _ = fmt.Fprintln(w)
	}

	// Ask for confirmation
	_, _ = fmt.Fprint(w, "Proceed with undo? (yes/no): ")

	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y", nil
}
