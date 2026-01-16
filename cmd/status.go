package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarlson/go-ralph/internal/config"
	"github.com/yarlson/go-ralph/internal/reporter"
	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current status",
		Long:  "Display task counts, next selected task, and last iteration outcome.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd)
		},
	}
}

func runStatus(cmd *cobra.Command) error {
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

	// Read parent task ID
	parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
	parentIDBytes, err := os.ReadFile(parentIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("parent-task-id file not found. Run 'ralph init' first")
		}
		return fmt.Errorf("failed to read parent-task-id: %w", err)
	}
	parentTaskID := string(parentIDBytes)

	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return fmt.Errorf("failed to open task store: %w", err)
	}

	// Validate parent task exists
	_, err = store.Get(parentTaskID)
	if err != nil {
		return fmt.Errorf("parent task %q not found: %w", parentTaskID, err)
	}

	// Get logs and state directories
	logsDir := state.LogsDirPath(workDir)
	stateDir := state.StateDirPath(workDir)

	// Create status generator
	generator := reporter.NewStatusGeneratorWithStateDir(store, logsDir, stateDir)

	// Get status
	status, err := generator.GetStatus(parentTaskID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Format and output
	_, _ = fmt.Fprint(cmd.OutOrStdout(), reporter.FormatStatus(status))

	return nil
}
