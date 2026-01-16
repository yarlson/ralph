package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

// GetConfigFile returns the config file path from the flag
func GetConfigFile() string {
	return cfgFile
}

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
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newPauseCmd())
	rootCmd.AddCommand(newResumeCmd())
	rootCmd.AddCommand(newRetryCmd())
	rootCmd.AddCommand(newSkipCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newRevertCmd())

	return rootCmd
}

// Execute runs the root command
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

