package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	cmdinternal "github.com/yarlson/ralph/cmd/internal"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/state"
)

var cfgFile string

// GetConfigFile returns the config file path from the flag
func GetConfigFile() string {
	return cfgFile
}

// Root command flags
var (
	rootOnce          bool
	rootMaxIterations int
	rootParent        string
	rootBranch        string
	rootDryRun        bool
)

// NewRootCmd creates the root command for ralph CLI
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ralph [file]",
		Short: "Ralph Wiggum Loop Harness for autonomous feature delivery",
		Long: `Ralph is a Go-based harness that orchestrates Claude Code for autonomous,
iterative feature delivery. It executes a "Ralph Wiggum loop": select ready task →
delegate to Claude Code → verify → commit → repeat.

Optionally, you can provide a file argument:
  - A PRD .md file to decompose into tasks
  - A task .yaml file to import tasks`,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runRoot,
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "ralph.yaml",
		"config file (default is ralph.yaml)")

	// Root command flags for file handling
	rootCmd.Flags().BoolVarP(&rootOnce, "once", "1", false,
		"run only a single iteration")
	rootCmd.Flags().IntVarP(&rootMaxIterations, "max-iterations", "n", 0,
		"maximum iterations to run (0 uses config default)")
	rootCmd.Flags().StringVarP(&rootParent, "parent", "p", "",
		"explicit parent task ID")
	rootCmd.Flags().StringVarP(&rootBranch, "branch", "b", "",
		"git branch override")
	rootCmd.Flags().BoolVar(&rootDryRun, "dry-run", false,
		"show what would be done without executing")

	// Add subcommands
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newDecomposeCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newPauseCmd())
	rootCmd.AddCommand(newResumeCmd())
	rootCmd.AddCommand(newRetryCmd())
	rootCmd.AddCommand(newSkipCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newRevertCmd())
	rootCmd.AddCommand(newFixCmd())

	return rootCmd
}

// runRoot handles the root command execution with optional file argument
func runRoot(cmd *cobra.Command, args []string) error {
	// If no file argument provided, auto-initialize and run
	if len(args) == 0 {
		return runRootAutoInit(cmd)
	}

	// File argument provided - validate and detect type
	filePath := args[0]

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Read file content for type detection
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect file type using bootstrap.DetectFileType
	fileType := cmdinternal.DetectFileType(string(content))

	switch fileType {
	case cmdinternal.FileTypePRD:
		if rootDryRun {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would decompose PRD file: %s\n", filePath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Detected file type: prd\n")
			if rootParent != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Parent task: %s\n", rootParent)
			}
			if rootBranch != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Branch: %s\n", rootBranch)
			}
			if rootOnce {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would run once\n")
			}
			if rootMaxIterations > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Max iterations: %d\n", rootMaxIterations)
			}
			return nil
		}
		// Run full PRD bootstrap pipeline: decompose → import → init → run
		opts := &cmdinternal.BootstrapOptions{
			Once:          rootOnce,
			MaxIterations: rootMaxIterations,
			Parent:        rootParent,
			Branch:        rootBranch,
		}
		return runBootstrapFromPRD(cmd, filePath, opts)

	case cmdinternal.FileTypeTasks:
		if rootDryRun {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would import task file: %s\n", filePath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Detected file type: tasks\n")
			if rootParent != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Parent task: %s\n", rootParent)
			}
			if rootBranch != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Branch: %s\n", rootBranch)
			}
			if rootOnce {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would run once\n")
			}
			if rootMaxIterations > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Max iterations: %d\n", rootMaxIterations)
			}
			return nil
		}
		// Run YAML bootstrap pipeline: import → init → run (no decompose)
		opts := &cmdinternal.BootstrapOptions{
			Once:          rootOnce,
			MaxIterations: rootMaxIterations,
			Parent:        rootParent,
			Branch:        rootBranch,
		}
		return runBootstrapFromYAML(cmd, filePath, opts)

	default:
		return fmt.Errorf("unknown file type: cannot determine if %s is a PRD or task file", filePath)
	}
}

// runRootAutoInit handles auto-initialization when ralph is run without a file argument
func runRootAutoInit(cmd *cobra.Command) error {
	// If explicit parent is provided, write parent-task-id file first
	if rootParent != "" {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Load configuration to get parent-task-id file location
		cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Ensure ralph directories exist
		if err := state.EnsureRalphDir(workDir); err != nil {
			return fmt.Errorf("failed to create .ralph directory: %w", err)
		}

		// Write parent-task-id file
		parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
		if err := os.WriteFile(parentIDFile, []byte(rootParent), 0644); err != nil {
			return fmt.Errorf("failed to write parent-task-id: %w", err)
		}

		// Also write to state directory
		if err := state.SetStoredParentTaskID(workDir, rootParent); err != nil {
			return fmt.Errorf("failed to set stored parent task ID: %w", err)
		}
	}

	// Delegate to runRun with the root flags
	// runRun already handles auto-initialization
	return runRun(cmd, rootOnce, rootMaxIterations, rootBranch)
}

// Execute runs the root command
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
