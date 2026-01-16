package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yarlson/go-ralph/internal/claude"
	"github.com/yarlson/go-ralph/internal/git"
	"github.com/yarlson/go-ralph/internal/memory"
	"github.com/yarlson/go-ralph/internal/prompt"
	"github.com/yarlson/go-ralph/internal/selector"
	"github.com/yarlson/go-ralph/internal/state"
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
	WorkDir      string
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
	workDir      string

	budget *BudgetTracker
	gutter *GutterDetector

	lastCompleted          *taskstore.Task
	maxRetries             int
	maxVerificationRetries int
	taskAttempts           map[string]int // tracks attempt count per task ID
}

// NewController creates a new loop controller with the given dependencies.
func NewController(deps ControllerDeps) *Controller {
	return &Controller{
		taskStore:              deps.TaskStore,
		claudeRunner:           deps.Claude,
		verifier:               deps.Verifier,
		gitManager:             deps.Git,
		logsDir:                deps.LogsDir,
		progressDir:            deps.ProgressDir,
		progressFile:           deps.ProgressFile,
		workDir:                deps.WorkDir,
		budget:                 NewBudgetTracker(DefaultBudgetLimits()),
		gutter:                 NewGutterDetector(DefaultGutterConfig()),
		maxRetries:             2, // default
		maxVerificationRetries: 2, // default
		taskAttempts:           make(map[string]int),
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

// SetMaxRetries sets the maximum number of retries per task.
func (c *Controller) SetMaxRetries(maxRetries int) {
	c.maxRetries = maxRetries
}

// SetMaxVerificationRetries sets the maximum number of verification retries within an iteration.
func (c *Controller) SetMaxVerificationRetries(maxVerificationRetries int) {
	c.maxVerificationRetries = maxVerificationRetries
}

// checkPaused checks if the loop has been paused by reading the pause flag file.
func (c *Controller) checkPaused() bool {
	if c.workDir == "" {
		return false
	}
	paused, err := state.IsPaused(c.workDir)
	if err != nil {
		return false
	}
	return paused
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

		// Check if loop has been paused
		if c.checkPaused() {
			result.Outcome = RunOutcomePaused
			result.Message = "loop paused (use 'ralph resume' to continue)"
			result.ElapsedTime = time.Since(startTime)
			return result
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
			// Mark the last attempted task as blocked if it exists
			if c.lastCompleted != nil {
				// Find the task that's stuck (last in-progress task)
				tasks, err := c.taskStore.List()
				if err == nil {
					for _, t := range tasks {
						if t.Status == taskstore.StatusInProgress {
							_ = c.taskStore.UpdateStatus(t.ID, taskstore.StatusBlocked)
							break
						}
					}
				}
			}
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

// runIteration executes a single task iteration with in-iteration retry loop for verification failures.
func (c *Controller) runIteration(ctx context.Context, task *taskstore.Task) *IterationRecord {
	record := NewIterationRecord(task.ID)

	// Track attempt number
	c.taskAttempts[task.ID]++
	record.AttemptNumber = c.taskAttempts[task.ID]

	// Get base commit
	baseCommit, err := c.gitManager.GetCurrentCommit(ctx)
	if err == nil {
		record.BaseCommit = baseCommit
	}

	// Mark task as in progress
	_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusInProgress)

	// Build prompt for Claude
	systemPrompt, userPrompt, err := c.buildPrompt(ctx, task)
	if err != nil {
		record.Complete(OutcomeFailed)
		record.SetFeedback(fmt.Sprintf("Failed to build prompt: %v", err))
		c.handleTaskFailure(task.ID)
		return record
	}

	// Invoke Claude (initial attempt)
	req := claude.ClaudeRequest{
		SystemPrompt: systemPrompt,
		Prompt:       userPrompt,
	}

	resp, err := c.claudeRunner.Run(ctx, req)
	if err != nil {
		record.Complete(OutcomeFailed)
		record.SetFeedback(fmt.Sprintf("Claude invocation failed: %v", err))
		c.handleTaskFailure(task.ID)
		return record
	}

	// Record Claude metadata (accumulate costs across retries)
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
		c.handleTaskFailure(task.ID)
		return record
	}

	// Get changed files
	changedFiles, _ := c.gitManager.GetChangedFiles(ctx)
	record.FilesChanged = changedFiles

	// Run verification with retry loop
	var results []verifier.VerificationResult
	verificationPassed := false
	verificationAttempt := 1

	if len(task.Verify) > 0 {
		for verificationAttempt <= c.maxVerificationRetries+1 {
			// Run verification
			results, err = c.verifier.Verify(ctx, task.Verify)
			if err != nil {
				record.Complete(OutcomeFailed)
				record.SetFeedback(fmt.Sprintf("Verification error: %v", err))
				c.handleTaskFailure(task.ID)
				return record
			}

			// Convert to VerificationOutput
			record.VerificationOutputs = []VerificationOutput{} // Reset for each attempt
			for _, r := range results {
				record.VerificationOutputs = append(record.VerificationOutputs, VerificationOutput{
					Command:  r.Command,
					Passed:   r.Passed,
					Output:   r.Output,
					Duration: r.Duration,
				})
			}

			// Check if all passed
			if record.AllPassed() {
				verificationPassed = true
				break
			}

			// If this was the last allowed attempt, fail
			if verificationAttempt > c.maxVerificationRetries {
				break
			}

			// Build retry prompt with failure context
			systemPrompt, userPrompt, err = c.buildRetryPromptForVerificationFailure(ctx, task, results, verificationAttempt)
			if err != nil {
				// If we can't build retry prompt, fail with current results
				break
			}

			// Invoke Claude again with --continue to fix in same session
			retryReq := claude.ClaudeRequest{
				SystemPrompt: systemPrompt,
				Prompt:       userPrompt,
				Continue:     true, // Continue in the same session
			}

			retryResp, err := c.claudeRunner.Run(ctx, retryReq)
			if err != nil {
				// If retry fails, break and use current verification results
				break
			}

			// Accumulate Claude costs and tokens across retries
			record.ClaudeInvocation.TotalCostUSD += retryResp.TotalCostUSD
			record.ClaudeInvocation.InputTokens += retryResp.Usage.InputTokens
			record.ClaudeInvocation.OutputTokens += retryResp.Usage.OutputTokens

			// Update changed files (Claude may have modified more files)
			changedFiles, _ = c.gitManager.GetChangedFiles(ctx)
			record.FilesChanged = changedFiles

			verificationAttempt++
		}

		if !verificationPassed {
			record.Complete(OutcomeFailed)
			record.SetFeedback(c.formatVerificationFeedback(results))
			c.handleTaskFailure(task.ID)
			return record
		}
	}

	// Commit changes
	commitMsg := git.FormatCommitMessage(task.Title, record.IterationID)
	commitHash, err := c.gitManager.Commit(ctx, commitMsg)
	if err != nil {
		record.Complete(OutcomeFailed)
		record.SetFeedback(fmt.Sprintf("Commit failed: %v", err))
		c.handleTaskFailure(task.ID)
		return record
	}

	record.ResultCommit = commitHash

	// Mark task completed and reset attempt counter
	_ = c.taskStore.UpdateStatus(task.ID, taskstore.StatusCompleted)
	delete(c.taskAttempts, task.ID) // Clear attempt count on success

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

// buildPrompt constructs the prompt for Claude using the full iteration prompt builder.
// For retries (attemptNumber > 1), it uses the retry prompt builder with failure context.
func (c *Controller) buildPrompt(ctx context.Context, task *taskstore.Task) (string, string, error) {
	builder := prompt.NewBuilder(nil) // Use default size options

	// Check if this is a retry (attempt > 1)
	attemptNumber := c.taskAttempts[task.ID]
	if attemptNumber > 1 {
		// This is a retry - build retry prompt
		return c.buildRetryPrompt(ctx, task, attemptNumber, builder)
	}

	// Initial attempt - build regular iteration prompt
	return c.buildInitialPrompt(ctx, task, builder)
}

// buildInitialPrompt builds the prompt for the initial attempt.
func (c *Controller) buildInitialPrompt(ctx context.Context, task *taskstore.Task, builder *prompt.Builder) (string, string, error) {
	// Extract codebase patterns from progress file
	var patterns string
	if c.progressFile != nil {
		patterns, _ = c.progressFile.GetCodebasePatterns()
	}

	// Get git diff stat if there are changes
	var diffStat string
	var changedFiles []string
	hasChanges, _ := c.gitManager.HasChanges(ctx)
	if hasChanges {
		diffStat, _ = c.gitManager.GetDiffStat(ctx)
		changedFiles, _ = c.gitManager.GetChangedFiles(ctx)
	}

	// Build iteration context
	iterCtx := prompt.IterationContext{
		Task:             task,
		CodebasePatterns: patterns,
		DiffStat:         diffStat,
		ChangedFiles:     changedFiles,
	}

	// Build prompts using prompt builder
	result, err := builder.Build(iterCtx)
	if err != nil {
		return "", "", err
	}

	return result.SystemPrompt, result.UserPrompt, nil
}

// buildRetryPrompt builds the prompt for a retry attempt after verification failure (between iterations).
func (c *Controller) buildRetryPrompt(ctx context.Context, task *taskstore.Task, attemptNumber int, builder *prompt.Builder) (string, string, error) {
	// Load user feedback if it exists
	var userFeedback string
	if c.workDir != "" {
		feedbackPath := filepath.Join(state.StateDirPath(c.workDir), fmt.Sprintf("feedback-%s.txt", task.ID))
		if feedbackBytes, err := os.ReadFile(feedbackPath); err == nil {
			userFeedback = string(feedbackBytes)
		}
	}

	// Load the most recent iteration record to get failure output
	var failureOutput string
	var failureSignature string
	if records, err := LoadAllIterationRecords(c.logsDir); err == nil {
		// Find the most recent failed iteration for this task
		for i := len(records) - 1; i >= 0; i-- {
			if records[i].TaskID == task.ID && records[i].Outcome == OutcomeFailed {
				// Extract failure outputs and compute signature
				failureSignature = ComputeFailureSignature(records[i].VerificationOutputs)

				// Convert verification outputs to verifier results for trimming
				var results []verifier.VerificationResult
				for _, vo := range records[i].VerificationOutputs {
					results = append(results, verifier.VerificationResult{
						Command:  vo.Command,
						Passed:   vo.Passed,
						Output:   vo.Output,
						Duration: vo.Duration,
					})
				}

				// Trim the failure output
				failureOutput = verifier.TrimOutputForFeedback(results, verifier.DefaultTrimOptions())
				break
			}
		}
	}

	// Build retry context
	retryCtx := prompt.RetryContext{
		Task:             task,
		FailureOutput:    failureOutput,
		FailureSignature: failureSignature,
		UserFeedback:     userFeedback,
		AttemptNumber:    attemptNumber,
	}

	// Build retry prompts
	result, err := builder.BuildRetry(retryCtx)
	if err != nil {
		return "", "", err
	}

	return result.SystemPrompt, result.UserPrompt, nil
}

// buildRetryPromptForVerificationFailure builds the prompt for an in-iteration verification retry.
func (c *Controller) buildRetryPromptForVerificationFailure(ctx context.Context, task *taskstore.Task, results []verifier.VerificationResult, attemptNumber int) (string, string, error) {
	builder := prompt.NewBuilder(nil)

	// Load user feedback if it exists (unlikely for in-iteration retries but check anyway)
	var userFeedback string
	if c.workDir != "" {
		feedbackPath := filepath.Join(state.StateDirPath(c.workDir), fmt.Sprintf("feedback-%s.txt", task.ID))
		if feedbackBytes, err := os.ReadFile(feedbackPath); err == nil {
			userFeedback = string(feedbackBytes)
		}
	}

	// Compute failure signature from current results
	var verificationOutputs []VerificationOutput
	for _, r := range results {
		verificationOutputs = append(verificationOutputs, VerificationOutput{
			Command:  r.Command,
			Passed:   r.Passed,
			Output:   r.Output,
			Duration: r.Duration,
		})
	}
	failureSignature := ComputeFailureSignature(verificationOutputs)

	// Trim failure output
	failureOutput := verifier.TrimOutputForFeedback(results, verifier.DefaultTrimOptions())

	// Build retry context
	retryCtx := prompt.RetryContext{
		Task:             task,
		FailureOutput:    failureOutput,
		FailureSignature: failureSignature,
		UserFeedback:     userFeedback,
		AttemptNumber:    attemptNumber,
	}

	// Build retry prompts
	result, err := builder.BuildRetry(retryCtx)
	if err != nil {
		return "", "", err
	}

	return result.SystemPrompt, result.UserPrompt, nil
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

// handleTaskFailure handles a task failure, setting the appropriate status based on retry count.
func (c *Controller) handleTaskFailure(taskID string) {
	attempts := c.taskAttempts[taskID]
	// maxRetries is the number of retries allowed (not counting the initial attempt)
	// So if maxRetries=2, we allow: 1 initial + 2 retries = 3 total attempts
	if attempts > c.maxRetries {
		// Max retries exhausted - mark as failed
		_ = c.taskStore.UpdateStatus(taskID, taskstore.StatusFailed)
	} else {
		// Still have retries left - reset to open
		_ = c.taskStore.UpdateStatus(taskID, taskstore.StatusOpen)
	}
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
