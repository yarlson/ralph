package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarlson/go-ralph/internal/config"
	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func newRetryCmd() *cobra.Command {
	var taskID string
	var feedback string

	cmd := &cobra.Command{
		Use:   "retry",
		Short: "Retry a failed task",
		Long:  "Reset a task to open status and add feedback for the next attempt.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRetry(cmd, taskID, feedback)
		},
	}

	cmd.Flags().StringVar(&taskID, "task", "", "task ID to retry (required)")
	cmd.Flags().StringVar(&feedback, "feedback", "", "feedback to include in next attempt")

	return cmd
}

func runRetry(cmd *cobra.Command, taskID, feedback string) error {
	// Validate required flag
	if taskID == "" {
		return errors.New("--task flag is required")
	}

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

	// Validate task state - can only retry failed, blocked, or in_progress tasks
	if task.Status == taskstore.StatusCompleted {
		return fmt.Errorf("cannot retry task %q: task is completed", taskID)
	}
	if task.Status == taskstore.StatusSkipped {
		return fmt.Errorf("cannot retry task %q: task is skipped (use 'ralph skip --task %s' to unskip first)", taskID, taskID)
	}
	if task.Status == taskstore.StatusOpen {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q is already open\n", taskID)
		return nil
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

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q reset to open status\n", taskID)
	return nil
}
