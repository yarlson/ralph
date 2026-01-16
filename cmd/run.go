package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/yarlson/go-ralph/internal/claude"
	"github.com/yarlson/go-ralph/internal/config"
	gitpkg "github.com/yarlson/go-ralph/internal/git"
	"github.com/yarlson/go-ralph/internal/loop"
	"github.com/yarlson/go-ralph/internal/memory"
	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
	"github.com/yarlson/go-ralph/internal/verifier"
)

func newRunCmd() *cobra.Command {
	var once bool
	var maxIterations int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the iteration loop",
		Long:  "Execute the iteration loop until all tasks are done or limits are reached.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, once, maxIterations)
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "run only a single iteration")
	cmd.Flags().IntVar(&maxIterations, "max-iterations", 0, "maximum iterations to run (0 uses config default)")

	return cmd
}

func runRun(cmd *cobra.Command, once bool, maxIterations int) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check if paused
	paused, err := state.IsPaused(workDir)
	if err == nil && paused {
		return fmt.Errorf("ralph is paused. Use 'ralph resume' to continue")
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

	// Ensure ralph directories exist
	if err := state.EnsureRalphDir(workDir); err != nil {
		return fmt.Errorf("failed to create .ralph directory: %w", err)
	}

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

	// Set up dependencies
	logsDir := state.LogsDirPath(workDir)
	claudeLogsDir := state.ClaudeLogsDirPath(workDir)
	progressPath := filepath.Join(workDir, cfg.Memory.ProgressFile)

	// Create progress file if it doesn't exist
	progressFile := memory.NewProgressFile(progressPath)
	if !progressFile.Exists() {
		parentTask, _ := store.Get(parentTaskID)
		featureName := "Unknown Feature"
		if parentTask != nil {
			featureName = parentTask.Title
		}
		if err := progressFile.Init(featureName, parentTaskID); err != nil {
			return fmt.Errorf("failed to initialize progress file: %w", err)
		}
	}

	// Create Claude runner
	claudeCommand := "claude"
	var claudeArgs []string
	if len(cfg.Claude.Command) > 0 {
		claudeCommand = cfg.Claude.Command[0]
		// If command has multiple parts (e.g., ["claude", "code"]),
		// use the first as command and rest as base args
		if len(cfg.Claude.Command) > 1 {
			claudeArgs = append(claudeArgs, cfg.Claude.Command[1:]...)
		}
	}
	// Append configured args
	claudeArgs = append(claudeArgs, cfg.Claude.Args...)

	claudeRunner := claude.NewSubprocessRunner(claudeCommand, claudeLogsDir)
	if len(claudeArgs) > 0 {
		claudeRunner.WithBaseArgs(claudeArgs)
	}

	// Create verifier
	ver := verifier.NewCommandRunner(workDir)
	if len(cfg.Safety.AllowedCommands) > 0 {
		ver.SetAllowedCommands(cfg.Safety.AllowedCommands)
	}

	// Create git manager
	gitManager := gitpkg.NewShellManager(workDir, cfg.Repo.BranchPrefix)

	// Build controller dependencies
	deps := loop.ControllerDeps{
		TaskStore:    store,
		Claude:       claudeRunner,
		Verifier:     ver,
		Git:          gitManager,
		LogsDir:      logsDir,
		ProgressDir:  filepath.Dir(progressPath),
		ProgressFile: progressFile,
	}

	// Create controller
	controller := loop.NewController(deps)

	// Configure budget limits
	budgetLimits := loop.BudgetLimits{
		MaxIterations:        cfg.Loop.MaxIterations,
		MaxMinutesPerIteration: cfg.Loop.MaxMinutesPerIteration,
	}
	if maxIterations > 0 {
		budgetLimits.MaxIterations = maxIterations
	}
	controller.SetBudgetLimits(budgetLimits)

	// Configure gutter detection
	gutterConfig := loop.GutterConfig{
		MaxSameFailure:     cfg.Loop.Gutter.MaxSameFailure,
		MaxChurnIterations: cfg.Loop.Gutter.MaxChurnCommits,
		ChurnThreshold:     3, // Default
	}
	controller.SetGutterConfig(gutterConfig)

	// Configure max retries
	controller.SetMaxRetries(cfg.Loop.MaxRetries)

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\nReceived interrupt signal, stopping after current iteration...\n")
		cancel()
	}()

	// Run the loop
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Starting ralph loop for parent task: %s\n\n", parentTaskID)

	var result loop.RunResult
	if once {
		result = controller.RunOnce(ctx, parentTaskID)
	} else {
		result = controller.RunLoop(ctx, parentTaskID)
	}

	// Output result
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s", formatRunResult(result))

	// Return error if the outcome indicates failure
	if result.Outcome == loop.RunOutcomeError {
		return fmt.Errorf("loop failed: %s", result.Message)
	}

	return nil
}

// formatRunResult formats a RunResult for CLI output.
func formatRunResult(result loop.RunResult) string {
	output := fmt.Sprintf("## Run Result: %s\n\n", result.Outcome)
	output += fmt.Sprintf("**Message**: %s\n\n", result.Message)

	output += "### Summary\n"
	output += fmt.Sprintf("- Iterations: %d\n", result.IterationsRun)
	output += fmt.Sprintf("- Completed tasks: %d completed\n", len(result.CompletedTasks))
	if len(result.FailedTasks) > 0 {
		output += fmt.Sprintf("- Failed tasks: %d failed\n", len(result.FailedTasks))
	}

	if result.TotalCostUSD > 0 {
		output += fmt.Sprintf("- Total cost: $%.4f\n", result.TotalCostUSD)
	}

	if result.ElapsedTime > 0 {
		output += fmt.Sprintf("- Elapsed time: %s\n", result.ElapsedTime.Round(1000000000))
	}

	if len(result.CompletedTasks) > 0 {
		output += "\n### Completed Tasks\n"
		for _, taskID := range result.CompletedTasks {
			output += fmt.Sprintf("- %s\n", taskID)
		}
	}

	if len(result.FailedTasks) > 0 {
		output += "\n### Failed Tasks\n"
		for _, taskID := range result.FailedTasks {
			output += fmt.Sprintf("- %s\n", taskID)
		}
	}

	return output
}
