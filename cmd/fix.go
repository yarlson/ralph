package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	cmdinternal "github.com/yarlson/ralph/cmd/internal"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

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
		if !cmdinternal.IsInteractive(os.Stdin.Fd()) {
			return runFixNonTTYError(cmd)
		}
	}

	// Handle --retry flag
	if retryID != "" {
		return runFixRetry(cmd, retryID, feedback)
	}

	// Handle --skip flag
	if skipID != "" {
		return runFixSkip(cmd, skipID, reason)
	}

	// TODO: Implement other fix command logic (undo)
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
		return errors.New("Cannot skip completed task")
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
