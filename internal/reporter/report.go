package reporter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yarlson/ralph/internal/git"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/taskstore"
)

// CommitInfo contains information about a git commit produced during the feature.
type CommitInfo struct {
	// Hash is the commit hash.
	Hash string

	// Message is the commit message.
	Message string

	// TaskID is the ID of the task that produced this commit.
	TaskID string

	// Timestamp is when the commit was created.
	Timestamp time.Time
}

// TaskSummary contains summary information about a task.
type TaskSummary struct {
	// ID is the task identifier.
	ID string

	// Title is the task title.
	Title string

	// Outcome is the iteration outcome for this task (if applicable).
	Outcome string
}

// BlockedTaskSummary contains information about a blocked task with its reason.
type BlockedTaskSummary struct {
	// ID is the task identifier.
	ID string

	// Title is the task title.
	Title string

	// Reason explains why the task is blocked.
	Reason string
}

// Report contains the end-of-feature summary report.
type Report struct {
	// ParentTaskID is the ID of the parent task for this feature.
	ParentTaskID string

	// FeatureName is the name of the feature (from parent task title).
	FeatureName string

	// Commits lists all commits produced during the feature.
	Commits []CommitInfo

	// CompletedTasks lists all tasks that were completed.
	CompletedTasks []TaskSummary

	// BlockedTasks lists all tasks that are blocked with their reasons.
	BlockedTasks []BlockedTaskSummary

	// FailedTasks lists all tasks that failed.
	FailedTasks []TaskSummary

	// SkippedTasks lists all tasks that were skipped.
	SkippedTasks []TaskSummary

	// TotalIterations is the total number of iterations run.
	TotalIterations int

	// TotalCostUSD is the total cost incurred.
	TotalCostUSD float64

	// TotalDuration is the total time spent.
	TotalDuration time.Duration

	// StartTime is when the first iteration started.
	StartTime time.Time

	// EndTime is when the last iteration completed.
	EndTime time.Time
}

// ReportGenerator generates end-of-feature reports.
type ReportGenerator struct {
	taskStore  taskstore.Store
	logsDir    string
	gitManager git.Manager
}

// NewReportGenerator creates a new report generator.
func NewReportGenerator(store taskstore.Store, logsDir string, gitManager git.Manager) *ReportGenerator {
	return &ReportGenerator{
		taskStore:  store,
		logsDir:    logsDir,
		gitManager: gitManager,
	}
}

// GenerateReport creates a complete feature report for the given parent task.
func (g *ReportGenerator) GenerateReport(parentTaskID string) (*Report, error) {
	tasks, err := g.taskStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	report := &Report{
		ParentTaskID: parentTaskID,
	}

	// Find the parent task to get feature name
	taskByID := make(map[string]*taskstore.Task)
	for _, t := range tasks {
		taskByID[t.ID] = t
	}
	if parent, ok := taskByID[parentTaskID]; ok {
		report.FeatureName = parent.Title
	}

	// Gather all descendants using BFS
	descendants := g.gatherDescendants(tasks, parentTaskID)

	// Categorize tasks by status
	for _, t := range descendants {
		switch t.Status {
		case taskstore.StatusCompleted:
			report.CompletedTasks = append(report.CompletedTasks, TaskSummary{
				ID:      t.ID,
				Title:   t.Title,
				Outcome: string(t.Status),
			})
		case taskstore.StatusBlocked:
			report.BlockedTasks = append(report.BlockedTasks, BlockedTaskSummary{
				ID:     t.ID,
				Title:  t.Title,
				Reason: g.getBlockedReason(t, taskByID),
			})
		case taskstore.StatusFailed:
			report.FailedTasks = append(report.FailedTasks, TaskSummary{
				ID:      t.ID,
				Title:   t.Title,
				Outcome: string(t.Status),
			})
		case taskstore.StatusSkipped:
			report.SkippedTasks = append(report.SkippedTasks, TaskSummary{
				ID:      t.ID,
				Title:   t.Title,
				Outcome: string(t.Status),
			})
		}
	}

	// Load iteration records for commits, costs, and timing
	if g.logsDir != "" {
		records, err := loop.LoadAllIterationRecords(g.logsDir)
		if err == nil {
			report.TotalIterations = len(records)

			for _, record := range records {
				report.TotalCostUSD += record.ClaudeInvocation.TotalCostUSD

				// Track commits from successful iterations
				if record.ResultCommit != "" {
					commitInfo := CommitInfo{
						Hash:      record.ResultCommit,
						TaskID:    record.TaskID,
						Timestamp: record.EndTime,
					}

					// Fetch commit message if git manager is available
					if g.gitManager != nil {
						ctx := context.Background()
						msg, err := g.gitManager.GetCommitMessage(ctx, record.ResultCommit)
						if err == nil {
							commitInfo.Message = msg
						}
						// If error, leave message empty (don't fail report generation)
					}

					report.Commits = append(report.Commits, commitInfo)
				}

				// Track time range
				if report.StartTime.IsZero() || record.StartTime.Before(report.StartTime) {
					report.StartTime = record.StartTime
				}
				if record.EndTime.After(report.EndTime) {
					report.EndTime = record.EndTime
				}
			}

			// Calculate total duration
			if !report.StartTime.IsZero() && !report.EndTime.IsZero() {
				report.TotalDuration = report.EndTime.Sub(report.StartTime)
			}
		}
	}

	return report, nil
}

// gatherDescendants collects all descendant tasks of the given parent.
func (g *ReportGenerator) gatherDescendants(tasks []*taskstore.Task, parentID string) []*taskstore.Task {
	// Build parent-to-children map
	children := make(map[string][]*taskstore.Task)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		}
	}

	// BFS to gather all descendants
	var descendants []*taskstore.Task
	queue := children[parentID]
	for len(queue) > 0 {
		task := queue[0]
		queue = queue[1:]
		descendants = append(descendants, task)
		queue = append(queue, children[task.ID]...)
	}

	return descendants
}

// getBlockedReason determines why a task is blocked.
func (g *ReportGenerator) getBlockedReason(task *taskstore.Task, taskByID map[string]*taskstore.Task) string {
	if task.Status == taskstore.StatusBlocked {
		// Check for incomplete dependencies
		var incompleteDeps []string
		for _, depID := range task.DependsOn {
			dep, ok := taskByID[depID]
			if !ok {
				incompleteDeps = append(incompleteDeps, depID+" (not found)")
			} else if dep.Status != taskstore.StatusCompleted {
				incompleteDeps = append(incompleteDeps, depID+" ("+string(dep.Status)+")")
			}
		}
		if len(incompleteDeps) > 0 {
			return fmt.Sprintf("blocked: waiting for dependencies: %s", strings.Join(incompleteDeps, ", "))
		}
		return "blocked: marked as blocked"
	}
	return ""
}

// FormatReport formats a report for CLI display.
func FormatReport(report *Report) string {
	var sb strings.Builder

	sb.WriteString("# Feature Report\n\n")

	// Basic info
	_, _ = fmt.Fprintf(&sb, "**Parent Task:** %s\n", report.ParentTaskID)
	if report.FeatureName != "" {
		_, _ = fmt.Fprintf(&sb, "**Feature:** %s\n", report.FeatureName)
	}
	sb.WriteString("\n")

	// Summary stats
	sb.WriteString("## Summary\n\n")
	_, _ = fmt.Fprintf(&sb, "- **Iterations:** %d iterations\n", report.TotalIterations)
	_, _ = fmt.Fprintf(&sb, "- **Total Cost:** $%.2f\n", report.TotalCostUSD)
	if report.TotalDuration > 0 {
		_, _ = fmt.Fprintf(&sb, "- **Duration:** %s\n", formatDuration(report.TotalDuration))
	}
	if !report.StartTime.IsZero() {
		_, _ = fmt.Fprintf(&sb, "- **Started:** %s\n", report.StartTime.Format(time.RFC3339))
	}
	if !report.EndTime.IsZero() {
		_, _ = fmt.Fprintf(&sb, "- **Completed:** %s\n", report.EndTime.Format(time.RFC3339))
	}
	sb.WriteString("\n")

	// Commits
	sb.WriteString("## Commits\n\n")
	if len(report.Commits) == 0 {
		sb.WriteString("No commits produced.\n")
	} else {
		for _, commit := range report.Commits {
			hash := commit.Hash
			if len(hash) > 7 {
				hash = hash[:7]
			}
			_, _ = fmt.Fprintf(&sb, "- `%s` %s (task: %s)\n", hash, commit.Message, commit.TaskID)
		}
	}
	sb.WriteString("\n")

	// Completed tasks
	sb.WriteString("## Completed Tasks\n\n")
	if len(report.CompletedTasks) == 0 {
		sb.WriteString("No completed tasks.\n")
	} else {
		for _, task := range report.CompletedTasks {
			_, _ = fmt.Fprintf(&sb, "- [x] %s (%s)\n", task.Title, task.ID)
		}
	}
	sb.WriteString("\n")

	// Blocked tasks
	if len(report.BlockedTasks) > 0 {
		sb.WriteString("## Blocked Tasks\n\n")
		for _, task := range report.BlockedTasks {
			_, _ = fmt.Fprintf(&sb, "- [ ] %s (%s)\n", task.Title, task.ID)
			_, _ = fmt.Fprintf(&sb, "      Reason: %s\n", task.Reason)
		}
		sb.WriteString("\n")
	}

	// Failed tasks
	if len(report.FailedTasks) > 0 {
		sb.WriteString("## Failed Tasks\n\n")
		for _, task := range report.FailedTasks {
			_, _ = fmt.Fprintf(&sb, "- [!] %s (%s)\n", task.Title, task.ID)
		}
		sb.WriteString("\n")
	}

	// Skipped tasks
	if len(report.SkippedTasks) > 0 {
		sb.WriteString("## Skipped Tasks\n\n")
		for _, task := range report.SkippedTasks {
			_, _ = fmt.Fprintf(&sb, "- [-] %s (%s)\n", task.Title, task.ID)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	return fmt.Sprintf("%.1f hours", d.Hours())
}
