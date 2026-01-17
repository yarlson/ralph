package internal

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// RootTaskOption represents a root task available for selection.
type RootTaskOption struct {
	// ID is the task identifier.
	ID string
	// Title is the task title.
	Title string
}

// SelectRootTask handles root task selection for auto-initialization.
// If there's a single task, it auto-selects with a confirmation message.
// If there are multiple tasks and isTTY is true, shows interactive menu.
// If there are multiple tasks and isTTY is false, returns an error with --parent hint.
// If there are no tasks, returns an error with helpful message.
func SelectRootTask(w io.Writer, r io.Reader, tasks []RootTaskOption, isTTY bool) (*RootTaskOption, error) {
	// No tasks
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks: run ralph <prd.md> or ralph <tasks.yaml>")
	}

	// Single task - auto-select with confirmation
	if len(tasks) == 1 {
		_, _ = fmt.Fprintf(w, "Initializing: %s (%s)\n", tasks[0].Title, tasks[0].ID)
		return &tasks[0], nil
	}

	// Multiple tasks - non-TTY error
	if !isTTY {
		var taskList string
		for _, t := range tasks {
			taskList += fmt.Sprintf("  - %s (%s)\n", t.Title, t.ID)
		}
		return nil, fmt.Errorf("multiple root tasks found (non-TTY):\n%s\nUse --parent <task-id> to specify which task to use", taskList)
	}

	// Multiple tasks - interactive menu
	_, _ = fmt.Fprintf(w, "\nSelect a root task:\n\n")
	for i, t := range tasks {
		_, _ = fmt.Fprintf(w, "  %d) %s (%s)\n", i+1, t.Title, t.ID)
	}
	_, _ = fmt.Fprintf(w, "\nEnter number (1-%d) or 'q' to quit: ", len(tasks))

	// Read input
	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read selection: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	if response == "q" || response == "quit" {
		return nil, fmt.Errorf("task selection cancelled")
	}

	// Parse selection
	selection, err := strconv.Atoi(response)
	if err != nil || selection < 1 || selection > len(tasks) {
		return nil, fmt.Errorf("invalid selection: %q (expected 1-%d)", response, len(tasks))
	}

	return &tasks[selection-1], nil
}

// FixActionType represents the type of action to perform in fix interactive mode.
type FixActionType string

const (
	// FixActionRetry retries a failed task.
	FixActionRetry FixActionType = "retry"
	// FixActionSkip skips a task.
	FixActionSkip FixActionType = "skip"
	// FixActionUndo undoes an iteration.
	FixActionUndo FixActionType = "undo"
)

// FixAction represents an action to be executed from interactive mode.
type FixAction struct {
	// Type is the type of action to perform.
	Type FixActionType
	// TargetID is the task ID or iteration ID.
	TargetID string
	// Feedback is optional feedback for retry with feedback.
	Feedback string
}

// FixIssue represents a task with issues (failed or blocked).
type FixIssue struct {
	// TaskID is the task identifier.
	TaskID string
	// Title is the task title.
	Title string
	// Status is the task status (failed or blocked).
	Status string
	// Attempts is the number of retry attempts.
	Attempts int
}

// FixIteration represents a recent iteration.
type FixIteration struct {
	// IterationID is the iteration identifier.
	IterationID string
	// TaskID is the related task ID.
	TaskID string
	// Outcome is the iteration outcome.
	Outcome string
}

// EditorFunc is a function that opens an editor for feedback.
// It takes a task ID and returns the feedback string or error.
type EditorFunc func(taskID string) (string, error)

// ActionHandler is a function that executes a fix action.
type ActionHandler func(action *FixAction) error

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

// FixInteractiveMode runs the interactive fix mode.
// It displays issues and iterations, prompts for commands, and executes actions.
// Commands: r <id> (retry), s <id> (skip), u <id> (undo), rf <id> (retry with feedback), q (quit).
func FixInteractiveMode(w io.Writer, r io.Reader, issues []FixIssue, iterations []FixIteration, handler ActionHandler) error {
	// Use a nil editor function - this will cause rf to fail gracefully
	return FixInteractiveModeWithEditor(w, r, issues, iterations, handler, nil)
}

// FixInteractiveModeWithEditor runs the interactive fix mode with a custom editor function.
// This allows testing and customizing the feedback editor behavior.
func FixInteractiveModeWithEditor(w io.Writer, r io.Reader, issues []FixIssue, iterations []FixIteration, handler ActionHandler, editorFn EditorFunc) error {
	// Display issues list
	_, _ = fmt.Fprintln(w, "Issues:")
	if len(issues) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	} else {
		for _, issue := range issues {
			_, _ = fmt.Fprintf(w, "  - %s: %s [%s] (attempts: %d)\n", issue.TaskID, issue.Title, issue.Status, issue.Attempts)
		}
	}
	_, _ = fmt.Fprintln(w)

	// Display recent iterations
	_, _ = fmt.Fprintln(w, "Recent Iterations:")
	if len(iterations) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
	} else {
		for _, iter := range iterations {
			_, _ = fmt.Fprintf(w, "  - %s: task=%s outcome=%s\n", iter.IterationID, iter.TaskID, iter.Outcome)
		}
	}
	_, _ = fmt.Fprintln(w)

	// Display available commands
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  r <id>  - retry task")
	_, _ = fmt.Fprintln(w, "  s <id>  - skip task")
	_, _ = fmt.Fprintln(w, "  u <id>  - undo iteration")
	_, _ = fmt.Fprintln(w, "  rf <id> - retry with feedback (opens editor)")
	_, _ = fmt.Fprintln(w, "  q       - quit")
	_, _ = fmt.Fprintln(w)

	reader := bufio.NewReader(r)

	for {
		// Show prompt
		_, _ = fmt.Fprint(w, "> ")

		// Read command
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to read command: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse command
		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "q", "quit":
			return nil

		case "r", "retry":
			if len(parts) < 2 {
				_, _ = fmt.Fprintln(w, "Error: retry requires task ID. Usage: r <task-id>")
				continue
			}
			taskID := strings.TrimSpace(parts[1])
			if handler != nil {
				if err := handler(&FixAction{Type: FixActionRetry, TargetID: taskID}); err != nil {
					_, _ = fmt.Fprintf(w, "Error: %v\n", err)
				}
			}

		case "s", "skip":
			if len(parts) < 2 {
				_, _ = fmt.Fprintln(w, "Error: skip requires task ID. Usage: s <task-id>")
				continue
			}
			taskID := strings.TrimSpace(parts[1])
			if handler != nil {
				if err := handler(&FixAction{Type: FixActionSkip, TargetID: taskID}); err != nil {
					_, _ = fmt.Fprintf(w, "Error: %v\n", err)
				}
			}

		case "u", "undo":
			if len(parts) < 2 {
				_, _ = fmt.Fprintln(w, "Error: undo requires iteration ID. Usage: u <iteration-id>")
				continue
			}
			iterID := strings.TrimSpace(parts[1])
			if handler != nil {
				if err := handler(&FixAction{Type: FixActionUndo, TargetID: iterID}); err != nil {
					_, _ = fmt.Fprintf(w, "Error: %v\n", err)
				}
			}

		case "rf":
			if len(parts) < 2 {
				_, _ = fmt.Fprintln(w, "Error: retry with feedback requires task ID. Usage: rf <task-id>")
				continue
			}
			taskID := strings.TrimSpace(parts[1])

			// Get feedback from editor
			var feedback string
			if editorFn != nil {
				var editorErr error
				feedback, editorErr = editorFn(taskID)
				if editorErr != nil {
					_, _ = fmt.Fprintf(w, "Error opening editor: %v\n", editorErr)
					continue
				}
			} else {
				_, _ = fmt.Fprintln(w, "Error: editor not available")
				continue
			}

			if handler != nil {
				if err := handler(&FixAction{Type: FixActionRetry, TargetID: taskID, Feedback: feedback}); err != nil {
					_, _ = fmt.Fprintf(w, "Error: %v\n", err)
				}
			}

		default:
			_, _ = fmt.Fprintf(w, "Unknown command: %s. Use 'q' to quit.\n", cmd)
		}
	}
}
