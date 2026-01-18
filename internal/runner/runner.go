// Package runner provides loop execution for ralph.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/term"

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

// Options configures a run.
type Options struct {
	Once          bool
	MaxIterations int
	Branch        string
	Stream        bool // Stream Claude output to console
}

// Run executes the main iteration loop.
func Run(ctx context.Context, workDir string, cfg *config.Config, parentTaskID string, opts Options, stdout, stderr io.Writer) error {
	// Check if paused - auto-resume if so
	paused, err := state.IsPaused(workDir)
	if err == nil && paused {
		if err := state.SetPaused(workDir, false); err != nil {
			return fmt.Errorf("failed to auto-resume: %w", err)
		}

		tasksPath := filepath.Join(workDir, cfg.Tasks.Path)
		store, storeErr := taskstore.NewLocalStore(tasksPath)
		var taskTitle string
		if storeErr == nil {
			if parentTask, getErr := store.Get(parentTaskID); getErr == nil {
				taskTitle = parentTask.Title
			}
		}

		if taskTitle != "" {
			_, _ = fmt.Fprintf(stdout, "Resuming: %s (%s)\n", taskTitle, parentTaskID)
		} else {
			_, _ = fmt.Fprintf(stdout, "Resuming: (%s)\n", parentTaskID)
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
		if len(cfg.Claude.Command) > 1 {
			claudeArgs = append(claudeArgs, cfg.Claude.Command[1:]...)
		}
	}
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

	streamWriter := io.Writer(nil)
	if opts.Stream {
		streamWriter = stdout
	}

	// Build controller dependencies
	deps := loop.ControllerDeps{
		TaskStore:      store,
		Claude:         claudeRunner,
		Verifier:       ver,
		Git:            gitManager,
		LogsDir:        logsDir,
		ProgressDir:    filepath.Dir(progressPath),
		ProgressFile:   progressFile,
		WorkDir:        workDir,
		ProgressWriter: stdout,
		StreamWriter:   streamWriter,
	}

	// Create controller
	controller := loop.NewController(deps)

	// Configure budget limits
	budgetLimits := loop.BudgetLimits{
		MaxIterations:          cfg.Loop.MaxIterations,
		MaxMinutesPerIteration: cfg.Loop.MaxMinutesPerIteration,
	}
	if opts.MaxIterations > 0 {
		budgetLimits.MaxIterations = opts.MaxIterations
	}
	controller.SetBudgetLimits(budgetLimits)

	// Configure gutter detection
	gutterConfig := loop.GutterConfig{
		MaxSameFailure:     cfg.Loop.Gutter.MaxSameFailure,
		MaxChurnIterations: cfg.Loop.Gutter.MaxChurnCommits,
		ChurnThreshold:     3,
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
	if opts.Branch != "" {
		controller.SetBranchOverride(opts.Branch)
	}

	// Configure sandbox mode if enabled
	if cfg.Safety.Sandbox {
		controller.SetSandboxMode(cfg.Safety.Sandbox, cfg.Safety.AllowedCommands)
	}

	// Set up context with signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_, _ = fmt.Fprintf(stderr, "\nReceived interrupt signal, stopping after current iteration...\n")
		cancel()
	}()

	// Run the loop
	_, _ = fmt.Fprintf(stdout, "Starting ralph loop for parent task: %s\n\n", parentTaskID)

	var result loop.RunResult
	if opts.Once {
		result = controller.RunOnce(ctx, parentTaskID)
	} else {
		result = controller.RunLoop(ctx, parentTaskID)
	}

	// Output result
	_, _ = fmt.Fprintf(stdout, "\n%s", FormatRunResult(result))

	// Return error if the outcome indicates failure
	if result.Outcome == loop.RunOutcomeError {
		return fmt.Errorf("loop failed: %s", result.Message)
	}

	return nil
}

// FormatRunResult formats a RunResult for CLI output.
func FormatRunResult(result loop.RunResult) string {
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

// IsTerminal checks if the reader is a terminal.
func IsTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return true
}

// ValidateTaskHasReadyLeaves checks that a task has at least one ready leaf descendant.
func ValidateTaskHasReadyLeaves(store *taskstore.LocalStore, taskID string) error {
	allTasks, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	graph, err := selector.BuildGraph(allTasks)
	if err != nil {
		return fmt.Errorf("failed to build task graph: %w", err)
	}

	if cycle := graph.DetectCycle(); cycle != nil {
		return fmt.Errorf("task graph contains cycle")
	}

	descendants := getDescendantIDsOf(allTasks, taskID)
	readyLeaves := selector.GetReadyLeaves(allTasks, graph)

	for _, leaf := range readyLeaves {
		if descendants[leaf.ID] || leaf.ID == taskID {
			return nil
		}
	}

	task, err := store.Get(taskID)
	if err != nil {
		return fmt.Errorf("no ready tasks under task %q", taskID)
	}

	return fmt.Errorf("no ready tasks under root %q", task.Title)
}

func getDescendantIDsOf(tasks []*taskstore.Task, parentID string) map[string]bool {
	children := make(map[string][]string)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}

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
