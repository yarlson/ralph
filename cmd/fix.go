package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

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

	// TODO: Implement other fix command logic (retry, skip, undo)
	return nil
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
