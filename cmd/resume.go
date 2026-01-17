package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/internal/state"
)

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume the iteration loop",
		Long:  "Clear the pause flag to allow the loop to continue.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResume(cmd)
		},
	}
}

func runResume(cmd *cobra.Command) error {
	warnDeprecated(cmd.ErrOrStderr(), "resume")

	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check if paused
	paused, err := state.IsPaused(workDir)
	if err != nil {
		return err
	}

	if !paused {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ralph is not paused\n")
		return nil
	}

	// Clear paused state
	if err := state.SetPaused(workDir, false); err != nil {
		return fmt.Errorf("failed to resume: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ralph loop resumed. Use 'ralph run' to start.\n")
	return nil
}
