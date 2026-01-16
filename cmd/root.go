package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

// NewRootCmd creates the root command for ralph CLI
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ralph",
		Short: "Ralph Wiggum Loop Harness for autonomous feature delivery",
		Long: `Ralph is a Go-based harness that orchestrates Claude Code for autonomous,
iterative feature delivery. It executes a "Ralph Wiggum loop": select ready task →
delegate to Claude Code → verify → commit → repeat.`,
		SilenceUsage: true,
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "ralph.yaml",
		"config file (default is ralph.yaml)")

	// Add subcommands
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newPauseCmd())
	rootCmd.AddCommand(newResumeCmd())
	rootCmd.AddCommand(newRetryCmd())
	rootCmd.AddCommand(newSkipCmd())
	rootCmd.AddCommand(newReportCmd())

	return rootCmd
}

// Execute runs the root command
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// errNotImplemented is returned by stub commands
var errNotImplemented = errors.New("not implemented")


func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Pause the iteration loop",
		Long:  "Set a pause flag to stop the loop after the current iteration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("pause: %w", errNotImplemented)
		},
	}
}

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume the iteration loop",
		Long:  "Clear the pause flag to allow the loop to continue.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("resume: %w", errNotImplemented)
		},
	}
}

func newRetryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retry",
		Short: "Retry a failed task",
		Long:  "Reset a task to open status and add feedback for the next attempt.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("retry: %w", errNotImplemented)
		},
	}
}

func newSkipCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skip",
		Short: "Skip a task",
		Long:  "Mark a task as skipped so the loop can continue.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("skip: %w", errNotImplemented)
		},
	}
}

func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Generate feature report",
		Long:  "Generate an end-of-feature summary with commits, tasks, and statistics.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("report: %w", errNotImplemented)
		},
	}
}
