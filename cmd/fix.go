package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/cmd/tui"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/git"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

// Test helpers for forcing TTY mode in tests
var (
	forceTTYMu    sync.Mutex
	forceTTYValue *bool
)

// SetTTYForTesting allows tests to force TTY detection to a specific value.
// Pass true to simulate TTY mode, false to simulate non-TTY mode.
// The effect is reset to nil (normal behavior) when the test completes.
func SetTTYForTesting(value bool) {
	forceTTYMu.Lock()
	defer forceTTYMu.Unlock()
	forceTTYValue = &value
}

// isTTYForFix returns true if we should behave as if we're in a TTY.
// This checks the forced value first (for testing), then falls back to real detection.
func isTTYForFix() bool {
	forceTTYMu.Lock()
	defer forceTTYMu.Unlock()
	if forceTTYValue != nil {
		return *forceTTYValue
	}
	return tui.IsInteractive(os.Stdin.Fd())
}

func newFixCmd() *cobra.Command {
	var retryID string
	var skipID string
	var undoID string
	var feedback string
	var reason string
	var force bool
	var list bool

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Fix failed tasks or undo iterations",
		Long: `Fix command provides options to retry failed tasks, skip tasks, or undo iterations.

Examples:
  ralph fix --retry task-123        # Retry a failed task
  ralph fix --skip task-123         # Skip a task
  ralph fix --undo iteration-001    # Undo an iteration
  ralph fix --list                  # List fixable issues`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFix(cmd, retryID, skipID, undoID, feedback, reason, force, list)
		},
	}

	cmd.Flags().StringVarP(&retryID, "retry", "r", "", "task ID to retry")
	cmd.Flags().StringVarP(&skipID, "skip", "s", "", "task ID to skip")
	cmd.Flags().StringVarP(&undoID, "undo", "u", "", "iteration ID to undo")
	cmd.Flags().StringVarP(&feedback, "feedback", "f", "", "feedback message for retry")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for skipping")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list fixable issues")

	return cmd
}

func runFix(cmd *cobra.Command, retryID, skipID, undoID, feedback, reason string, force, list bool) error {
	// Handle --list flag
	if list {
		return runFixList(cmd)
	}

	// Check if any action flags are provided
	hasActionFlag := retryID != "" || skipID != "" || undoID != ""

	// If no action flags, check if we're in a TTY
	if !hasActionFlag {
		// Detect non-TTY by checking stdin
		if !isTTYForFix() {
			return runFixNonTTYError(cmd)
		}
		// Launch interactive mode for TTY with no flags
		return runFixInteractive(cmd, force)
	}

	// Handle --retry flag
	if retryID != "" {
		return runFixRetry(cmd, retryID, feedback)
	}

	// Handle --skip flag
	if skipID != "" {
		return runFixSkip(cmd, skipID, reason)
	}

	// Handle --undo flag
	if undoID != "" {
		return runFixUndo(cmd, undoID, force)
	}

	return nil
}

// runFixSkip marks a task as skipped.
func runFixSkip(cmd *cobra.Command, taskID, reason string) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Get the task
	task, err := store.Get(taskID)
	if err != nil {
		var notFoundErr *taskstore.NotFoundError
		if errors.As(err, &notFoundErr) {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Validate task state - can only skip open, failed, or blocked tasks
	switch task.Status {
	case taskstore.StatusOpen, taskstore.StatusFailed, taskstore.StatusBlocked:
		// OK to skip
	case taskstore.StatusSkipped:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q is already skipped\n", taskID)
		return nil
	case taskstore.StatusCompleted:
		return errors.New("cannot skip completed task")
	default:
		return fmt.Errorf("cannot skip task %q: task status is %q (must be open, failed, or blocked)", taskID, task.Status)
	}

	// Update task status to skipped
	if err := store.UpdateStatus(taskID, taskstore.StatusSkipped); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Save reason if provided
	if reason != "" {
		if err := state.EnsureRalphDir(workDir); err != nil {
			return fmt.Errorf("failed to ensure .ralph directory: %w", err)
		}

		reasonFile := filepath.Join(state.StateDirPath(workDir), fmt.Sprintf("skip-reason-%s.txt", taskID))
		if err := os.WriteFile(reasonFile, []byte(reason), 0644); err != nil {
			return fmt.Errorf("failed to write reason file: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skip reason saved for task %q\n", taskID)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q marked as skipped\n", taskID)
	return nil
}

// runFixRetry resets a task to open status for retry.
func runFixRetry(cmd *cobra.Command, taskID, feedback string) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Get the task
	task, err := store.Get(taskID)
	if err != nil {
		var notFoundErr *taskstore.NotFoundError
		if errors.As(err, &notFoundErr) {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Validate task state - can only retry failed or open tasks
	switch task.Status {
	case taskstore.StatusFailed:
		// OK to retry
	case taskstore.StatusOpen:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q is already open\n", taskID)
		return nil
	case taskstore.StatusCompleted:
		return fmt.Errorf("cannot retry task %q: task is completed", taskID)
	default:
		return fmt.Errorf("cannot retry task %q: task status is %q (must be failed or open)", taskID, task.Status)
	}

	// Reset task status to open
	if err := store.UpdateStatus(taskID, taskstore.StatusOpen); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Save feedback if provided
	if feedback != "" {
		if err := state.EnsureRalphDir(workDir); err != nil {
			return fmt.Errorf("failed to ensure .ralph directory: %w", err)
		}

		feedbackFile := filepath.Join(state.StateDirPath(workDir), fmt.Sprintf("feedback-%s.txt", taskID))
		if err := os.WriteFile(feedbackFile, []byte(feedback), 0644); err != nil {
			return fmt.Errorf("failed to write feedback file: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Feedback saved for task %q\n", taskID)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Retry initiated: task %q reset to open status\n", taskID)
	return nil
}

// runFixNonTTYError displays guidance and returns an error when fix is called
// without flags in a non-TTY environment.
func runFixNonTTYError(cmd *cobra.Command) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Get all tasks
	tasks, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Separate failed and blocked tasks
	var failedTasks []*taskstore.Task
	for _, task := range tasks {
		if task.Status == taskstore.StatusFailed {
			failedTasks = append(failedTasks, task)
		}
	}

	// Load iteration records
	logsDir := state.LogsDirPath(workDir)
	iterations, err := loop.LoadAllIterationRecords(logsDir)
	if err != nil {
		// Don't fail if logs directory doesn't exist
		iterations = nil
	}

	// Sort iterations by end time (most recent first)
	sort.Slice(iterations, func(i, j int) bool {
		return iterations[i].EndTime.After(iterations[j].EndTime)
	})

	// Limit to recent iterations (last 10)
	const maxIterations = 10
	if len(iterations) > maxIterations {
		iterations = iterations[:maxIterations]
	}

	// Output fixable issues with guidance
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Fixable Issues:")
	if len(failedTasks) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, task := range failedTasks {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", task.ID, task.Title)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    use: ralph fix --retry %s\n", task.ID)
		}
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	// Output recent iterations with guidance
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent Iterations:")
	if len(iterations) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, iter := range iterations {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: task=%s outcome=%s\n",
				iter.IterationID, iter.TaskID, iter.Outcome)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    use: ralph fix --undo %s\n", iter.IterationID)
		}
	}

	return errors.New("interactive mode requires TTY: use explicit flags (--retry, --skip, --undo, --list)")
}

// runFixList displays fixable issues in a structured format.
func runFixList(cmd *cobra.Command) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Get all tasks
	tasks, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Separate failed and blocked tasks
	var failedTasks []*taskstore.Task
	var blockedTasks []*taskstore.Task
	for _, task := range tasks {
		switch task.Status {
		case taskstore.StatusFailed:
			failedTasks = append(failedTasks, task)
		case taskstore.StatusBlocked:
			blockedTasks = append(blockedTasks, task)
		}
	}

	// Load iteration records
	logsDir := state.LogsDirPath(workDir)
	iterations, err := loop.LoadAllIterationRecords(logsDir)
	if err != nil {
		// Don't fail if logs directory doesn't exist
		iterations = nil
	}

	// Sort iterations by end time (most recent first)
	sort.Slice(iterations, func(i, j int) bool {
		return iterations[i].EndTime.After(iterations[j].EndTime)
	})

	// Limit to recent iterations (last 10)
	const maxIterations = 10
	if len(iterations) > maxIterations {
		iterations = iterations[:maxIterations]
	}

	// Output failed tasks
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Failed Tasks:")
	if len(failedTasks) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, task := range failedTasks {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", task.ID, task.Title)
		}
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	// Output blocked tasks
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Blocked Tasks:")
	if len(blockedTasks) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, task := range blockedTasks {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", task.ID, task.Title)
		}
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	// Output recent iterations
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent Iterations:")
	if len(iterations) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
	} else {
		for _, iter := range iterations {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s: task=%s outcome=%s\n",
				iter.IterationID, iter.TaskID, iter.Outcome)
		}
	}

	return nil
}

// runFixUndo reverts a specified iteration.
func runFixUndo(cmd *cobra.Command, iterationID string, force bool) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load iteration record
	logsDir := state.LogsDirPath(workDir)
	iterationFile := filepath.Join(logsDir, fmt.Sprintf("iteration-%s.json", iterationID))

	// Check if iteration exists
	if _, err := os.Stat(iterationFile); os.IsNotExist(err) {
		return errors.New("iteration not found")
	}

	record, err := loop.LoadRecord(iterationFile)
	if err != nil {
		return fmt.Errorf("failed to load iteration record: %w", err)
	}

	// Check if base commit is available
	if record.BaseCommit == "" {
		return fmt.Errorf("iteration %q has no base commit recorded", iterationID)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize git manager
	gitManager := git.NewShellManager(workDir, "")

	// Check for uncommitted changes
	ctx := cmd.Context()
	hasChanges, err := gitManager.HasChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	// Determine task to reopen (if any)
	taskToReopen := ""
	if record.Outcome == loop.OutcomeSuccess && record.TaskID != "" {
		tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
		store, err := taskstore.NewLocalStore(tasksPath)
		if err == nil {
			task, err := store.Get(record.TaskID)
			if err == nil && task.Status == taskstore.StatusCompleted {
				taskToReopen = record.TaskID
			}
		}
	}

	// Confirm with user unless --force
	if !force {
		confirmInfo := tui.UndoConfirmationInfo{
			IterationID:           iterationID,
			CommitToResetTo:       record.BaseCommit,
			TaskToReopen:          taskToReopen,
			FilesToRevert:         record.FilesChanged,
			HasUncommittedChanges: hasChanges,
		}

		confirmed, err := tui.ConfirmUndo(cmd.OutOrStdout(), cmd.InOrStdin(), confirmInfo)
		if err != nil {
			return err
		}
		if !confirmed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Undo cancelled.\n")
			return nil
		}
	}

	// Perform git reset --hard to base commit
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reverting to commit %s...\n", record.BaseCommit)
	if err := gitResetHardFix(workDir, record.BaseCommit); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}

	// Update task status if it was completed in this iteration
	if taskToReopen != "" {
		tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
		store, err := taskstore.NewLocalStore(tasksPath)
		if err != nil {
			return fmt.Errorf("failed to open task store: %w", err)
		}

		if err := store.UpdateStatus(taskToReopen, taskstore.StatusOpen); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q reset to open status\n", taskToReopen)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Undo completed: reverted iteration %s\n", iterationID)
	return nil
}

// gitResetHardFix performs a git reset --hard to the given commit.
func gitResetHardFix(workDir, commit string) error {
	cmd := exec.Command("git", "reset", "--hard", commit)
	cmd.Dir = workDir
	return cmd.Run()
}

// runFixInteractive runs the interactive fix mode.
// It shows issues, recent iterations, and prompts for commands.
func runFixInteractive(cmd *cobra.Command, force bool) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Get all tasks
	tasks, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Load iteration records
	logsDir := state.LogsDirPath(workDir)
	iterations, err := loop.LoadAllIterationRecords(logsDir)
	if err != nil {
		// Don't fail if logs directory doesn't exist
		iterations = nil
	}

	// Build issues list (failed and blocked tasks)
	var issues []tui.FixIssue
	for _, task := range tasks {
		if task.Status == taskstore.StatusFailed || task.Status == taskstore.StatusBlocked {
			// Count attempts from iteration records
			attempts := countTaskAttempts(iterations, task.ID)
			issues = append(issues, tui.FixIssue{
				TaskID:   task.ID,
				Title:    task.Title,
				Status:   string(task.Status),
				Attempts: attempts,
			})
		}
	}

	// Sort iterations by end time (most recent first) and limit to 10
	sort.Slice(iterations, func(i, j int) bool {
		return iterations[i].EndTime.After(iterations[j].EndTime)
	})
	const maxIterations = 10
	if len(iterations) > maxIterations {
		iterations = iterations[:maxIterations]
	}

	// Build iterations list
	var fixIterations []tui.FixIteration
	for _, iter := range iterations {
		fixIterations = append(fixIterations, tui.FixIteration{
			IterationID: iter.IterationID,
			TaskID:      iter.TaskID,
			Outcome:     string(iter.Outcome),
		})
	}

	// Create action handler
	handler := func(action *tui.FixAction) error {
		return executeFixAction(cmd, workDir, cfg, store, action, force)
	}

	// Create editor function for retry with feedback
	editorFn := func(taskID string) (string, error) {
		return openEditorForFeedback(taskID)
	}

	// Run interactive mode
	return tui.FixInteractiveModeWithEditor(
		cmd.OutOrStdout(),
		cmd.InOrStdin(),
		issues,
		fixIterations,
		handler,
		editorFn,
	)
}

// countTaskAttempts counts the number of attempts for a task from iteration records.
func countTaskAttempts(iterations []*loop.IterationRecord, taskID string) int {
	count := 0
	for _, iter := range iterations {
		if iter.TaskID == taskID {
			count++
		}
	}
	return count
}

// executeFixAction executes a fix action from interactive mode.
func executeFixAction(cmd *cobra.Command, workDir string, cfg *config.Config, store *taskstore.LocalStore, action *tui.FixAction, force bool) error {
	switch action.Type {
	case tui.FixActionRetry:
		return runFixRetry(cmd, action.TargetID, action.Feedback)
	case tui.FixActionSkip:
		return runFixSkip(cmd, action.TargetID, "")
	case tui.FixActionUndo:
		return runFixUndo(cmd, action.TargetID, force)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// openEditorForFeedback opens the user's editor for entering feedback.
// It creates a temporary file with instructions and returns the content.
func openEditorForFeedback(taskID string) (string, error) {
	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors
		for _, e := range []string{"vim", "vi", "nano", "notepad"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return "", fmt.Errorf("no editor found. Set EDITOR or VISUAL environment variable")
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("ralph-feedback-%s-*.txt", taskID))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// Write instructions to file
	instructions := fmt.Sprintf(`# Enter feedback for task %s
# Lines starting with # will be ignored.
# Save and close the editor to continue.

`, taskID)
	if _, err := tmpFile.WriteString(instructions); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to write instructions: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Open editor
	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Read feedback from file
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read feedback: %w", err)
	}

	// Parse content, removing comment lines
	return parseEditorContent(string(content)), nil
}

// parseEditorContent removes comment lines and trims whitespace from editor content.
func parseEditorContent(content string) string {
	var result []byte
	inComment := false
	lineStart := true

	for _, ch := range content {
		if lineStart && ch == '#' {
			inComment = true
		}
		if ch == '\n' {
			if !inComment {
				result = append(result, '\n')
			}
			inComment = false
			lineStart = true
		} else {
			lineStart = false
			if !inComment {
				result = append(result, byte(ch))
			}
		}
	}

	// Trim leading and trailing whitespace
	return trimWhitespace(string(result))
}

// trimWhitespace removes leading and trailing whitespace from a string.
func trimWhitespace(s string) string {
	start := 0
	end := len(s)

	// Find first non-whitespace
	for start < end && isWhitespace(s[start]) {
		start++
	}

	// Find last non-whitespace
	for end > start && isWhitespace(s[end-1]) {
		end--
	}

	return s[start:end]
}

// isWhitespace returns true if the byte is whitespace.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// Ensure io is used (for future interface compatibility)
var _ io.Writer = os.Stdout
