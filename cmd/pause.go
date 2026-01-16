package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/internal/state"
)

func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Pause the iteration loop",
		Long:  "Set a pause flag to stop the loop after the current iteration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPause(cmd)
		},
	}
}

func runPause(cmd *cobra.Command) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check if already paused
	paused, err := state.IsPaused(workDir)
	if err != nil {
		return err
	}

	if paused {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ralph is already paused\n")
		return nil
	}

	// Set paused
	if err := state.SetPaused(workDir, true); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ralph loop paused. Use 'ralph resume' to continue.\n")
	return nil
}
