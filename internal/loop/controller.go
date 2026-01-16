package loop

import (
	"context"
	"fmt"
	"time"

	"github.com/yarlson/go-ralph/internal/claude"
	"github.com/yarlson/go-ralph/internal/git"
	"github.com/yarlson/go-ralph/internal/memory"
	"github.com/yarlson/go-ralph/internal/selector"
	"github.com/yarlson/go-ralph/internal/taskstore"
	"github.com/yarlson/go-ralph/internal/verifier"
)

// RunLoopOutcome represents the final outcome of a loop run.
type RunLoopOutcome string

const (
	// RunOutcomeCompleted indicates all tasks were completed successfully.
	RunOutcomeCompleted RunLoopOutcome = "completed"
	// RunOutcomeBlocked indicates no ready tasks were available.
	RunOutcomeBlocked RunLoopOutcome = "blocked"
	// RunOutcomeBudgetExceeded indicates the budget limit was reached.
	RunOutcomeBudgetExceeded RunLoopOutcome = "budget_exceeded"
	// RunOutcomeGutterDetected indicates a gutter condition was detected.
	RunOutcomeGutterDetected RunLoopOutcome = "gutter_detected"
	// RunOutcomePaused indicates the loop was paused or cancelled.
	RunOutcomePaused RunLoopOutcome = "paused"
	// RunOutcomeError indicates a fatal error occurred.
	RunOutcomeError RunLoopOutcome = "error"
)

// validRunOutcomes is the set of valid run outcomes.
var validRunOutcomes = map[RunLoopOutcome]bool{
	RunOutcomeCompleted:      true,
	RunOutcomeBlocked:        true,
	RunOutcomeBudgetExceeded: true,
	RunOutcomeGutterDetected: true,
	RunOutcomePaused:         true,
	RunOutcomeError:          true,
}

// IsValid returns true if the outcome is a valid value.
func (o RunLoopOutcome) IsValid() bool {
	return validRunOutcomes[o]
}

// RunResult contains the results from a loop run.
type RunResult struct {
	// Outcome is the final outcome of the run.
	Outcome RunLoopOutcome

	// Message is a human-readable description of the outcome.
	Message string

	// IterationsRun is the number of iterations completed.
	IterationsRun int

	// CompletedTasks is the list of task IDs that were completed.
	CompletedTasks []string

	// FailedTasks is the list of task IDs that failed.
	FailedTasks []string

	// Records contains the iteration records from the run.
	Records []*IterationRecord

	// TotalCostUSD is the total cost across all iterations.
	TotalCostUSD float64

	// ElapsedTime is the total time for the run.
	ElapsedTime time.Duration
}

// Summary provides an overview of task status for a parent task.
type Summary struct {
	ParentTaskID   string
	TotalCount     int
	CompletedCount int
	OpenCount      int
	BlockedCount   int
	FailedCount    int
	SkippedCount   int
	NextTask       *taskstore.Task
}

// ControllerDeps contains the dependencies for the Controller.
type ControllerDeps struct {
	TaskStore    taskstore.Store
	Claude       claude.Runner
	Verifier     verifier.Verifier
	Git          git.Manager
	LogsDir      string
	ProgressDir  string
	ProgressFile *memory.ProgressFile
}

// Controller orchestrates the main iteration loop.
type Controller struct {
	taskStore    taskstore.Store
	claudeRunner claude.Runner
	verifier     verifier.Verifier
	gitManager   git.Manager
	logsDir      string
	progressDir  string
	progressFile *memory.ProgressFile

	budget *BudgetTracker
	gutter *GutterDetector

	lastCompleted *taskstore.Task
}

// NewController creates a new loop controller with the given dependencies.
func NewController(deps ControllerDeps) *Controller {
	return &Controller{
		taskStore:    deps.TaskStore,
		claudeRunner: deps.Claude,
		verifier:     deps.Verifier,
		gitManager:   deps.Git,
		logsDir:      deps.LogsDir,
		progressDir:  deps.ProgressDir,
		progressFile: deps.ProgressFile,
		budget:       NewBudgetTracker(DefaultBudgetLimits()),
		gutter:       NewGutterDetector(DefaultGutterConfig()),
	}
}

// SetBudgetLimits sets the budget limits for the controller.
func (c *Controller) SetBudgetLimits(limits BudgetLimits) {
	c.budget = NewBudgetTracker(limits)
}

// SetGutterConfig sets the gutter detection configuration.
func (c *Controller) SetGutterConfig(config GutterConfig) {
	c.gutter = NewGutterDetector(config)
}

// RunLoop executes the main iteration loop until completion, blocked, or budget exceeded.
func (c *Controller) RunLoop(ctx context.Context, parentTaskID string) RunResult {
	startTime := time.Now()
	result := RunResult{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
		Records:        []*IterationRecord{},
	}

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Outcome = RunOutcomePaused
			result.Message = "loop cancelled"
			result.ElapsedTime = time.Since(startTime)
			return result
		default:
		}

		// Check budget before iteration
		budgetStatus := c.budget.CheckBudget()
		if !budgetStatus.CanContinue {
			result.Outcome = RunOutcomeBudgetExceeded
			result.Message = budgetStatus.Reason
			result.ElapsedTime = time.Since(startTime)
			return result
		}

		// Check gutter before iteration
		gutterStatus := c.gutter.Check()
		if gutterStatus.InGutter {
			result.Outcome = RunOutcomeGutterDetected
			result.Message = gutterStatus.Description
			result.ElapsedTime = time.Since(startTime)
			return result
		}

		// Get tasks and select next
		tasks, err := c.taskStore.List()
		if err != nil {
			result.Outcome = RunOutcomeError
			result.Message = fmt.Sprintf("failed to list tasks: %v", err)
			result.ElapsedTime = time.Since(startTime)
			return result
		}

		graph, err := selector.BuildGraph(tasks)
		if err != nil {
			result.Outcome = RunOutcomeError
			result.Message = fmt.Sprintf("failed to build dependency graph: %v", err)
			result.ElapsedTime = time.Since(startTime)
			return result
		}

		nextTask := selector.SelectNext(tasks, graph, parentTaskID, c.lastCompleted)
		if nextTask == nil {
			// No more ready tasks - either completed or blocked
			result.Outcome = RunOutcomeCompleted
			result.Message = "all tasks completed"

			// Check if there are any incomplete tasks
			for _, t := range tasks {
				if t.ParentID != nil && *t.ParentID == parentTaskID {
					if t.Status == taskstore.StatusOpen || t.Status == taskstore.StatusInProgress {
						result.Outcome = RunOutcomeBlocked
						result.Message = "no ready tasks available (some tasks may be blocked by dependencies)"
						break
					}
				}
			}

			// Also check descendants
			if result.Outcome == RunOutcomeCompleted {
				hasIncomplete := c.hasIncompleteDescendants(tasks, parentTaskID)
				if hasIncomplete {
					result.Outcome = RunOutcomeBlocked
					result.Message = "no ready tasks available (blocked by incomplete dependencies)"
				}
			}

			result.ElapsedTime = time.Since(startTime)
			return result
		}

		// Run single iteration
		record := c.runIteration(ctx, nextTask)
		result.Records = append(result.Records, record)
		result.IterationsRun++
		result.TotalCostUSD += record.ClaudeInvocation.TotalCostUSD

		// Track in budget and gutter
		c.budget.RecordIteration(record.ClaudeInvocation.TotalCostUSD)
		c.gutter.RecordIteration(record)

		// Handle outcome
		if record.Outcome == OutcomeSuccess {
			result.CompletedTasks = append(result.CompletedTasks, nextTask.ID)
			c.lastCompleted = nextTask
		} else {
			result.FailedTasks = append(result.FailedTasks, nextTask.ID)
		}

		// Save iteration record
		_, _ = SaveRecord(c.logsDir, record)
	}
}

// RunOnce executes a single iteration and returns.
func (c *Controller) RunOnce(ctx context.Context, parentTaskID string) RunResult {
	startTime := time.Now()
	result := RunResult{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
		Records:        []*IterationRecord{},
	}

	// Check context
	select {
	case <-ctx.Done():
		result.Outcome = RunOutcomePaused
		result.Message = "cancelled"
		result.ElapsedTime = time.Since(startTime)
		return result
	default:
	}

	// Get tasks and select next
	tasks, err := c.taskStore.List()
	if err != nil {
		result.Outcome = RunOutcomeError
		result.Message = fmt.Sprintf("failed to list tasks: %v", err)
		result.ElapsedTime = time.Since(startTime)
		return result
	}

	graph, err := selector.BuildGraph(tasks)
	if err != nil {
		result.Outcome = RunOutcomeError
		result.Message = fmt.Sprintf("failed to build dependency graph: %v", err)
		result.ElapsedTime = time.Since(startTime)
		return result
	}

	nextTask := selector.SelectNext(tasks, graph, parentTaskID, c.lastCompleted)
	if nextTask == nil {
		result.Outcome = RunOutcomeBlocked
		result.Message = "no ready tasks available"
		result.ElapsedTime = time.Since(startTime)
		return result
	}

	// Run iteration
	record := c.runIteration(ctx, nextTask)
	result.Records = append(result.Records, record)
	result.IterationsRun = 1
	result.TotalCostUSD = record.ClaudeInvocation.TotalCostUSD

	if record.Outcome == OutcomeSuccess {
		result.Outcome = RunOutcomeCompleted
		result.Message = "iteration completed successfully"
		result.CompletedTasks = append(result.CompletedTasks, nextTask.ID)
		c.lastCompleted = nextTask
	} else {
		result.Outcome = RunOutcomeBlocked
		result.Message = "iteration failed"
		result.FailedTasks = append(result.FailedTasks, nextTask.ID)
	}

	// Save record
	_, _ = SaveRecord(c.logsDir, record)

	result.ElapsedTime = time.Since(startTime)
	return result
}

// runIteration executes a single task iteration.
func (c *Controller) runIteration(ctx context.Context, task *taskstore.Task) *IterationRecord {
	record := NewIterationRecord(task.ID)

	// Get base commit
	baseCommit, err := c.gitManager.GetCurrentCommit(ctx)
	if err == nil {
		record.BaseCommit = baseCommit
	}

	// Mark task as in progress
	_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusInProgress)

	// Build prompt for Claude
	prompt := c.buildPrompt(task)

	// Invoke Claude
	req := claude.ClaudeRequest{
		Prompt: prompt,
	}

	resp, err := c.claudeRunner.Run(ctx, req)
	if err != nil {
		record.Complete(OutcomeFailed)
		record.SetFeedback(fmt.Sprintf("Claude invocation failed: %v", err))
		_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusOpen) // Reset to open
		return record
	}

	// Record Claude metadata
	record.ClaudeInvocation = ClaudeInvocationMeta{
		SessionID:    resp.SessionID,
		Model:        resp.Model,
		TotalCostUSD: resp.TotalCostUSD,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}

	// Check for changes
	hasChanges, err := c.gitManager.HasChanges(ctx)
	if err != nil || !hasChanges {
		record.Complete(OutcomeFailed)
		record.SetFeedback("No changes made by Claude")
		_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusOpen)
		return record
	}

	// Get changed files
	changedFiles, _ := c.gitManager.GetChangedFiles(ctx)
	record.FilesChanged = changedFiles

	// Run verification
	if len(task.Verify) > 0 {
		results, err := c.verifier.Verify(ctx, task.Verify)
		if err != nil {
			record.Complete(OutcomeFailed)
			record.SetFeedback(fmt.Sprintf("Verification error: %v", err))
			_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusOpen)
			return record
		}

		// Convert to VerificationOutput
		for _, r := range results {
			record.VerificationOutputs = append(record.VerificationOutputs, VerificationOutput{
				Command:  r.Command,
				Passed:   r.Passed,
				Output:   r.Output,
				Duration: r.Duration,
			})
		}

		// Check if all passed
		if !record.AllPassed() {
			record.Complete(OutcomeFailed)
			record.SetFeedback(c.formatVerificationFeedback(results))
			_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusOpen)
			return record
		}
	}

	// Commit changes
	commitMsg := git.FormatCommitMessage(task.Title, record.IterationID)
	commitHash, err := c.gitManager.Commit(ctx, commitMsg)
	if err != nil {
		record.Complete(OutcomeFailed)
		record.SetFeedback(fmt.Sprintf("Commit failed: %v", err))
		_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusOpen)
		return record
	}

	record.ResultCommit = commitHash

	// Mark task completed
	_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusCompleted)

	// Update progress file
	if c.progressFile != nil {
		entry := memory.IterationEntry{
			TaskID:       task.ID,
			TaskTitle:    task.Title,
			WhatChanged:  []string{resp.FinalText},
			FilesTouched: changedFiles,
			Outcome:      "Success",
		}
		_ = c.progressFile.AppendIteration(entry)
	}

	record.Complete(OutcomeSuccess)
	return record
}

// buildPrompt constructs the prompt for Claude.
func (c *Controller) buildPrompt(task *taskstore.Task) string {
	var prompt string

	prompt = fmt.Sprintf("## Task: %s\n\n", task.Title)
	prompt += fmt.Sprintf("### Description\n%s\n\n", task.Description)

	if len(task.Acceptance) > 0 {
		prompt += "### Acceptance Criteria\n"
		for _, a := range task.Acceptance {
			prompt += fmt.Sprintf("- %s\n", a)
		}
		prompt += "\n"
	}

	if len(task.Verify) > 0 {
		prompt += "### Verification Commands\n"
		prompt += "Run these commands to verify your changes:\n"
		for _, v := range task.Verify {
			prompt += fmt.Sprintf("- `%v`\n", v)
		}
		prompt += "\n"
	}

	prompt += "### Instructions\n"
	prompt += "1. Implement the task according to the description and acceptance criteria.\n"
	prompt += "2. Run the verification commands and fix any failures.\n"
	prompt += "3. Do not commit - the harness will commit after verification.\n"

	return prompt
}

// formatVerificationFeedback formats verification failures for retry feedback.
func (c *Controller) formatVerificationFeedback(results []verifier.VerificationResult) string {
	feedback := "Verification failed:\n"
	for _, r := range results {
		if !r.Passed {
			feedback += fmt.Sprintf("\nCommand: %v\nOutput:\n%s\n", r.Command, r.Output)
		}
	}
	return feedback
}

// hasIncompleteDescendants checks if there are any incomplete tasks under the parent.
func (c *Controller) hasIncompleteDescendants(tasks []*taskstore.Task, parentID string) bool {
	// Build parent-to-children map
	children := make(map[string][]*taskstore.Task)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		}
	}

	// BFS to check all descendants
	queue := children[parentID]
	for len(queue) > 0 {
		task := queue[0]
		queue = queue[1:]

		if task.Status == taskstore.StatusOpen ||
			task.Status == taskstore.StatusInProgress ||
			task.Status == taskstore.StatusBlocked {
			return true
		}

		queue = append(queue, children[task.ID]...)
	}

	return false
}

// GetSummary returns a summary of task status for the given parent task.
func (c *Controller) GetSummary(ctx context.Context, parentTaskID string) (*Summary, error) {
	tasks, err := c.taskStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	summary := &Summary{
		ParentTaskID: parentTaskID,
	}

	// Build parent-to-children map
	children := make(map[string][]*taskstore.Task)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		}
	}

	// BFS to count all descendants
	queue := children[parentTaskID]
	for len(queue) > 0 {
		task := queue[0]
		queue = queue[1:]

		summary.TotalCount++

		switch task.Status {
		case taskstore.StatusCompleted:
			summary.CompletedCount++
		case taskstore.StatusOpen, taskstore.StatusInProgress:
			summary.OpenCount++
		case taskstore.StatusBlocked:
			summary.BlockedCount++
		case taskstore.StatusFailed:
			summary.FailedCount++
		case taskstore.StatusSkipped:
			summary.SkippedCount++
		}

		queue = append(queue, children[task.ID]...)
	}

	// Get next task
	graph, err := selector.BuildGraph(tasks)
	if err == nil {
		summary.NextTask = selector.SelectNext(tasks, graph, parentTaskID, c.lastCompleted)
	}

	return summary, nil
}
