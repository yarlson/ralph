package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarlson/ralph/cmd/tui"
	"github.com/yarlson/ralph/internal/bootstrap"
	"github.com/yarlson/ralph/internal/config"
	"github.com/yarlson/ralph/internal/detect"
	"github.com/yarlson/ralph/internal/runner"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

var cfgFile string

// GetConfigFile returns the config file path from the flag.
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
	rootStream        bool
	rootProvider      string
)

// NewRootCmd creates the root command for ralph CLI.
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/ralph/config.yaml)")
	rootCmd.Flags().BoolVarP(&rootOnce, "once", "1", false, "run only a single iteration")
	rootCmd.Flags().IntVarP(&rootMaxIterations, "max-iterations", "n", 0, "maximum iterations (0 uses config)")
	rootCmd.Flags().StringVarP(&rootParent, "parent", "p", "", "explicit parent task ID")
	rootCmd.Flags().StringVarP(&rootBranch, "branch", "b", "", "git branch override")
	rootCmd.Flags().BoolVar(&rootDryRun, "dry-run", false, "show what would be done")
	rootCmd.Flags().BoolVar(&rootStream, "stream", false, "stream agent output to console")
	rootCmd.PersistentFlags().StringVar(&rootProvider, "provider", "", "LLM provider (claude or opencode)")

	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newFixCmd())

	return rootCmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runRootAutoInit(cmd)
	}

	filePath := args[0]
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	fileType := detect.DetectFileType(string(content))

	switch fileType {
	case detect.FileTypePRD:
		return runPRDBootstrap(cmd, filePath)
	case detect.FileTypeTasks:
		return runYAMLBootstrap(cmd, filePath)
	default:
		return fmt.Errorf("unknown file type: cannot determine if %s is a PRD or task file", filePath)
	}
}

func runRootAutoInit(cmd *cobra.Command) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if rootParent != "" {
		if err := state.EnsureRalphDir(workDir); err != nil {
			return fmt.Errorf("failed to create .ralph directory: %w", err)
		}
		parentIDFile := filepath.Join(workDir, config.DefaultParentIDFile)
		if err := os.WriteFile(parentIDFile, []byte(rootParent), 0644); err != nil {
			return fmt.Errorf("failed to write parent-task-id: %w", err)
		}
		if err := state.SetStoredParentTaskID(workDir, rootParent); err != nil {
			return fmt.Errorf("failed to set stored parent task ID: %w", err)
		}
	}

	parentIDFile := filepath.Join(workDir, config.DefaultParentIDFile)
	parentIDBytes, err := os.ReadFile(parentIDFile)
	var parentTaskID string

	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No parent task set. Attempting auto-initialization...\n")
			autoInitID, wasAutoInit, autoErr := autoInitParentTask(cmd, workDir, cfg)
			if autoErr != nil {
				return autoErr
			}
			if wasAutoInit {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Auto-initialized with parent task: %s\n\n", autoInitID)
			}
			parentTaskID = autoInitID
		} else {
			return fmt.Errorf("failed to read parent-task-id: %w", err)
		}
	} else {
		parentTaskID = string(parentIDBytes)
	}

	opts := runner.Options{
		Once:          rootOnce,
		MaxIterations: rootMaxIterations,
		Branch:        rootBranch,
		Stream:        rootStream,
		Provider:      rootProvider,
	}

	return runner.Run(cmd.Context(), workDir, cfg, parentTaskID, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func runPRDBootstrap(cmd *cobra.Command, prdPath string) error {
	if rootDryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would decompose PRD file: %s\n", prdPath)
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

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	opts := bootstrap.Options{
		Once:          rootOnce,
		MaxIterations: rootMaxIterations,
		Parent:        rootParent,
		Branch:        rootBranch,
		Stream:        rootStream,
		Provider:      rootProvider,
	}

	return bootstrap.RunFromPRD(cmd.Context(), prdPath, workDir, cfg, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func runYAMLBootstrap(cmd *cobra.Command, yamlPath string) error {
	if rootDryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would import task file: %s\n", yamlPath)
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

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	opts := bootstrap.Options{
		Once:          rootOnce,
		MaxIterations: rootMaxIterations,
		Parent:        rootParent,
		Branch:        rootBranch,
		Stream:        rootStream,
		Provider:      rootProvider,
	}

	return bootstrap.RunFromYAML(cmd.Context(), yamlPath, workDir, cfg, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func autoInitParentTask(cmd *cobra.Command, workDir string, cfg *config.Config) (string, bool, error) {
	tasksPath := filepath.Join(workDir, config.DefaultTasksPath)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to open task store: %w", err)
	}

	rootTasks, err := store.ListByParent("")
	if err != nil {
		return "", false, fmt.Errorf("failed to list root tasks: %w", err)
	}

	options := make([]tui.RootTaskOption, len(rootTasks))
	for i, t := range rootTasks {
		options[i] = tui.RootTaskOption{ID: t.ID, Title: t.Title}
	}

	isTTY := runner.IsTerminal(cmd.InOrStdin())
	selected, err := tui.SelectRootTask(cmd.OutOrStdout(), cmd.InOrStdin(), options, isTTY)
	if err != nil {
		return "", false, err
	}

	selectedTask, err := store.Get(selected.ID)
	if err != nil {
		return "", false, fmt.Errorf("failed to get selected task: %w", err)
	}

	if err := runner.ValidateTaskHasReadyLeaves(store, selectedTask.ID); err != nil {
		return "", false, err
	}

	if err := state.EnsureRalphDir(workDir); err != nil {
		return "", false, fmt.Errorf("failed to create .ralph directory: %w", err)
	}

	parentIDFile := filepath.Join(workDir, config.DefaultParentIDFile)
	if err := os.WriteFile(parentIDFile, []byte(selectedTask.ID), 0644); err != nil {
		return "", false, fmt.Errorf("failed to write parent-task-id: %w", err)
	}

	if err := state.SetStoredParentTaskID(workDir, selectedTask.ID); err != nil {
		return "", false, fmt.Errorf("failed to set stored parent task ID: %w", err)
	}

	return selectedTask.ID, true, nil
}

// Execute runs the root command.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
