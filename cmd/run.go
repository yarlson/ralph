package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	cmdinternal "github.com/yarlson/ralph/cmd/internal"
	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/config"
	gitpkg "github.com/yarlson/ralph/internal/git"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/memory"
	"github.com/yarlson/ralph/internal/selector"
	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
	"github.com/yarlson/ralph/internal/verifier"
)

func newRunCmd() *cobra.Command {
	var once bool
	var maxIterations int
	var branch string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the iteration loop",
		Long:  "Execute the iteration loop until all tasks are done or limits are reached.",
		RunE: func(cmd *cobra.Command, args []string) error {
			warnDeprecated(cmd.ErrOrStderr(), "run")
			return runRun(cmd, once, maxIterations, branch)
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "run only a single iteration")
	cmd.Flags().IntVar(&maxIterations, "max-iterations", 0, "maximum iterations to run (0 uses config default)")
	cmd.Flags().StringVar(&branch, "branch", "", "override branch name (default: auto-generate from parent task)")

	return cmd
}

func runRun(cmd *cobra.Command, once bool, maxIterations int, branch string) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration first (needed for parent task lookup)
	cfg, err := config.LoadConfigWithFile(workDir, GetConfigFile())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Read parent task ID (or auto-initialize)
	parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
	parentIDBytes, err := os.ReadFile(parentIDFile)
	var parentTaskID string

	if err != nil {
		if os.IsNotExist(err) {
			// Attempt auto-initialization
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

	// Check if paused - auto-resume if so
	paused, err := state.IsPaused(workDir)
	if err == nil && paused {
		// Auto-resume: clear paused state and show resuming message
		if err := state.SetPaused(workDir, false); err != nil {
			return fmt.Errorf("failed to auto-resume: %w", err)
		}

		// Get parent task title for the message
		tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
		store, storeErr := taskstore.NewLocalStore(tasksPath)
		var taskTitle string
		if storeErr == nil {
			if parentTask, getErr := store.Get(parentTaskID); getErr == nil {
				taskTitle = parentTask.Title
			}
		}

		if taskTitle != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resuming: %s (%s)\n", taskTitle, parentTaskID)
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resuming: (%s)\n", parentTaskID)
		}
	}

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

	// Create verifier with sandbox mode enforcement if enabled
	ver := verifier.NewCommandRunner(workDir)
	if cfg.Safety.Sandbox && len(cfg.Safety.AllowedCommands) > 0 {
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
		WorkDir:      workDir,
	}

	// Create controller
	controller := loop.NewController(deps)

	// Configure budget limits
	budgetLimits := loop.BudgetLimits{
		MaxIterations:          cfg.Loop.MaxIterations,
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

	// Configure memory limits
	controller.SetMemoryConfig(cfg.Memory.MaxProgressBytes, cfg.Memory.MaxRecentIterations)

	// Configure max retries
	controller.SetMaxRetries(cfg.Loop.MaxRetries)
	controller.SetMaxVerificationRetries(cfg.Loop.MaxVerificationRetries)

	// Configure config-level verification commands
	controller.SetConfigVerifyCommands(cfg.Verification.Commands)

	// Configure branch override if specified
	if branch != "" {
		controller.SetBranchOverride(branch)
	}

	// Configure sandbox mode if enabled
	if cfg.Safety.Sandbox {
		controller.SetSandboxMode(cfg.Safety.Sandbox, cfg.Safety.AllowedCommands)
	}

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

// autoInitParentTask attempts automatic parent task initialization
// Returns: (parentTaskID, wasAutoInit, error)
func autoInitParentTask(cmd *cobra.Command, workDir string, cfg *config.Config) (string, bool, error) {
	// Open task store
	tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
	store, err := taskstore.NewLocalStore(tasksPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to open task store: %w", err)
	}

	// Get root tasks
	rootTasks, err := store.ListByParent("")
	if err != nil {
		return "", false, fmt.Errorf("failed to list root tasks: %w", err)
	}

	// Convert to RootTaskOption slice for SelectRootTask
	options := make([]cmdinternal.RootTaskOption, len(rootTasks))
	for i, t := range rootTasks {
		options[i] = cmdinternal.RootTaskOption{ID: t.ID, Title: t.Title}
	}

	// Determine if stdin is a TTY
	isTTY := isTerminal(cmd.InOrStdin())

	// Use SelectRootTask for unified selection logic
	selected, err := cmdinternal.SelectRootTask(cmd.OutOrStdout(), cmd.InOrStdin(), options, isTTY)
	if err != nil {
		return "", false, err
	}

	// Find the full task object
	selectedTask, err := store.Get(selected.ID)
	if err != nil {
		return "", false, fmt.Errorf("failed to get selected task: %w", err)
	}

	// Validate selected task has ready leaves
	if err := validateTaskHasReadyLeaves(store, selectedTask.ID); err != nil {
		return "", false, err
	}

	// Ensure state directory exists
	if err := state.EnsureRalphDir(workDir); err != nil {
		return "", false, fmt.Errorf("failed to create .ralph directory: %w", err)
	}

	// Write parent-task-id file (config-specified location for backward compat)
	parentIDFile := filepath.Join(workDir, cfg.Tasks.ParentIDFile)
	if err := os.WriteFile(parentIDFile, []byte(selectedTask.ID), 0644); err != nil {
		return "", false, fmt.Errorf("failed to write parent-task-id: %w", err)
	}

	// Write to state directory
	if err := state.SetStoredParentTaskID(workDir, selectedTask.ID); err != nil {
		return "", false, fmt.Errorf("failed to set stored parent task ID: %w", err)
	}

	// Initialize progress file if needed
	progressPath := filepath.Join(workDir, cfg.Memory.ProgressFile)
	progressFile := memory.NewProgressFile(progressPath)
	if !progressFile.Exists() {
		if err := progressFile.Init(selectedTask.Title, selectedTask.ID); err != nil {
			return "", false, fmt.Errorf("failed to initialize progress file: %w", err)
		}
	}

	return selectedTask.ID, true, nil
}

// isTerminal checks if the reader is a terminal (for interactive prompts)
func isTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	// For testing: if it's not a file (e.g., bytes.Buffer), assume it's interactive
	// This allows tests to mock stdin input
	return true
}

// validateTaskHasReadyLeaves checks that a task has at least one ready leaf descendant
func validateTaskHasReadyLeaves(store *taskstore.LocalStore, taskID string) error {
	// Load all tasks
	allTasks, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	// Build graph
	graph, err := selector.BuildGraph(allTasks)
	if err != nil {
		return fmt.Errorf("failed to build task graph: %w", err)
	}

	// Check for cycles
	if cycle := graph.DetectCycle(); cycle != nil {
		return fmt.Errorf("task graph contains cycle")
	}

	// Get descendants of selected root
	descendants := getDescendantIDsOf(allTasks, taskID)

	// Get ready leaves
	readyLeaves := selector.GetReadyLeaves(allTasks, graph)

	// Check if any ready leaf is a descendant
	for _, leaf := range readyLeaves {
		if descendants[leaf.ID] || leaf.ID == taskID {
			return nil // Found at least one ready task
		}
	}

	// Get the task to show its title in error
	task, err := store.Get(taskID)
	if err != nil {
		return fmt.Errorf("no ready tasks under task %q", taskID)
	}

	return fmt.Errorf("no ready tasks under root %q", task.Title)
}

// getDescendantIDsOf returns a set of all descendant task IDs under the given parent
func getDescendantIDsOf(tasks []*taskstore.Task, parentID string) map[string]bool {
	// Build parent-to-children map
	children := make(map[string][]string)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}

	// BFS to find all descendants
	descendants := make(map[string]bool)
	queue := children[parentID]

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		descendants[id] = true
		queue = append(queue, children[id]...)
	}

	return descendants
}
