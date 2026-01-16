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

func newSkipCmd() *cobra.Command {
	var taskID string
	var reason string

	cmd := &cobra.Command{
		Use:   "skip",
		Short: "Skip a task",
		Long:  "Mark a task as skipped so the loop can continue.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkip(cmd, taskID, reason)
		},
	}

	cmd.Flags().StringVar(&taskID, "task", "", "task ID to skip (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for skipping the task")

	return cmd
}

func runSkip(cmd *cobra.Command, taskID, reason string) error {
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
	cfg, err := config.LoadConfig(workDir)
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

	// Validate task state - cannot skip completed tasks
	if task.Status == taskstore.StatusCompleted {
		return fmt.Errorf("cannot skip task %q: task is already completed", taskID)
	}
	if task.Status == taskstore.StatusSkipped {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q is already skipped\n", taskID)
		return nil
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
