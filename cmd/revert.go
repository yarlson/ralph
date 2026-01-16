package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/git"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

func newRevertCmd() *cobra.Command {
	var iterationID string
	var force bool

	cmd := &cobra.Command{
		Use:   "revert",
		Short: "Revert to before a specific iteration",
		Long: `Revert the repository to the state before a specific iteration.
This performs a git reset --hard to the base commit of the iteration
and updates task status back to open if the task was completed.

WARNING: This will discard all uncommitted changes!`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevert(cmd, iterationID, force)
		},
	}

	cmd.Flags().StringVar(&iterationID, "iteration", "", "iteration ID to revert (required)")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompt")

	return cmd
}

func runRevert(cmd *cobra.Command, iterationID string, force bool) error {
	// Validate required flag
	if iterationID == "" {
		return errors.New("--iteration flag is required")
	}

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
		return fmt.Errorf("iteration %q not found", iterationID)
	}

	record, err := loop.LoadRecord(iterationFile)
	if err != nil {
		return fmt.Errorf("failed to load iteration record: %w", err)
	}

	// Check if base commit is available
	if record.BaseCommit == "" {
		return fmt.Errorf("iteration %q has no base commit recorded", iterationID)
	}

	// Confirm with user unless --force
	if !force {
		confirmed, err := confirmRevert(cmd, record)
		if err != nil {
			return err
		}
		if !confirmed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Revert cancelled.\n")
			return nil
		}
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
	if hasChanges {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "WARNING: You have uncommitted changes that will be lost!\n")
	}

	// Perform git reset --hard to base commit
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reverting to commit %s...\n", record.BaseCommit)
	if err := gitResetHard(workDir, record.BaseCommit); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}

	// Update task status if it was completed in this iteration
	if record.Outcome == loop.OutcomeSuccess && record.TaskID != "" {
		tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
		store, err := taskstore.NewLocalStore(tasksPath)
		if err != nil {
			return fmt.Errorf("failed to open task store: %w", err)
		}

		task, err := store.Get(record.TaskID)
		if err != nil {
			// Task might not exist, that's ok
			var notFoundErr *taskstore.NotFoundError
			if !errors.As(err, &notFoundErr) {
				return fmt.Errorf("failed to get task: %w", err)
			}
		} else if task.Status == taskstore.StatusCompleted {
			// Reset to open
			if err := store.UpdateStatus(record.TaskID, taskstore.StatusOpen); err != nil {
				return fmt.Errorf("failed to update task status: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %q reset to open status\n", record.TaskID)
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Successfully reverted to before iteration %s\n", iterationID)
	return nil
}

func confirmRevert(cmd *cobra.Command, record *loop.IterationRecord) (bool, error) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "This will revert to commit %s (before iteration %s)\n",
		record.BaseCommit, record.IterationID)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task: %s\n", record.TaskID)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Outcome: %s\n", record.Outcome)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nWARNING: This will discard all uncommitted changes!\n\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Are you sure? (yes/no): ")

	reader := bufio.NewReader(cmd.InOrStdin())
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y", nil
}

func gitResetHard(workDir, commit string) error {
	cmd := exec.Command("git", "reset", "--hard", commit)
	cmd.Dir = workDir
	return cmd.Run()
}
