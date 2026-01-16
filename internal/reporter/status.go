// Package reporter provides status display and report generation for the Ralph harness.
package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yarlson/go-ralph/internal/loop"
	"github.com/yarlson/go-ralph/internal/selector"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

// TaskCounts holds the count of tasks in each status.
type TaskCounts struct {
	// Total is the total number of descendant tasks under the parent.
	Total int

	// Completed is the count of tasks with status "completed".
	Completed int

	// Ready is the count of tasks that are ready to execute (open, all deps completed, is leaf).
	Ready int

	// Blocked is the count of tasks with status "blocked".
	Blocked int

	// Failed is the count of tasks with status "failed".
	Failed int

	// Skipped is the count of tasks with status "skipped".
	Skipped int
}

// LastIterationInfo contains summary information about the last iteration.
type LastIterationInfo struct {
	// IterationID is the unique identifier of the iteration.
	IterationID string

	// TaskID is the ID of the task that was executed.
	TaskID string

	// TaskTitle is the title of the task that was executed.
	TaskTitle string

	// Outcome is the result of the iteration.
	Outcome loop.IterationOutcome

	// EndTime is when the iteration completed.
	EndTime time.Time

	// LogPath is the path to the iteration log file.
	LogPath string
}

// Status contains all status information for a parent task.
type Status struct {
	// ParentTaskID is the ID of the parent task being reported on.
	ParentTaskID string

	// Counts holds the task counts by status.
	Counts TaskCounts

	// NextTask is the next task that will be executed (if any).
	NextTask *taskstore.Task

	// LastIteration contains info about the most recent iteration (if any).
	LastIteration *LastIterationInfo
}

// StatusGenerator generates status information for a parent task.
type StatusGenerator struct {
	taskStore taskstore.Store
	logsDir   string
}

// NewStatusGenerator creates a new status generator.
func NewStatusGenerator(store taskstore.Store, logsDir string) *StatusGenerator {
	return &StatusGenerator{
		taskStore: store,
		logsDir:   logsDir,
	}
}

// GetStatus returns the current status for the given parent task ID.
func (g *StatusGenerator) GetStatus(parentTaskID string) (*Status, error) {
	tasks, err := g.taskStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	status := &Status{
		ParentTaskID: parentTaskID,
	}

	// Build parent-to-children map for traversal
	children := make(map[string][]*taskstore.Task)
	taskByID := make(map[string]*taskstore.Task)
	for _, t := range tasks {
		taskByID[t.ID] = t
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		}
	}

	// Gather all descendants
	descendants := make([]*taskstore.Task, 0)
	queue := children[parentTaskID]
	for len(queue) > 0 {
		task := queue[0]
		queue = queue[1:]
		descendants = append(descendants, task)
		queue = append(queue, children[task.ID]...)
	}

	// Count tasks by status
	status.Counts.Total = len(descendants)
	for _, t := range descendants {
		switch t.Status {
		case taskstore.StatusCompleted:
			status.Counts.Completed++
		case taskstore.StatusBlocked:
			status.Counts.Blocked++
		case taskstore.StatusFailed:
			status.Counts.Failed++
		case taskstore.StatusSkipped:
			status.Counts.Skipped++
		}
	}

	// Build dependency graph and count ready tasks
	graph, err := selector.BuildGraph(tasks)
	if err == nil {
		readyLeaves := selector.GetReadyLeaves(descendants, graph)
		status.Counts.Ready = len(readyLeaves)

		// Get next task
		status.NextTask = selector.SelectNext(tasks, graph, parentTaskID, nil)
	}

	// Load last iteration info
	if g.logsDir != "" {
		record, path, err := FindLatestIterationRecord(g.logsDir)
		if err == nil && record != nil {
			taskTitle := ""
			if task, ok := taskByID[record.TaskID]; ok {
				taskTitle = task.Title
			}
			status.LastIteration = &LastIterationInfo{
				IterationID: record.IterationID,
				TaskID:      record.TaskID,
				TaskTitle:   taskTitle,
				Outcome:     record.Outcome,
				EndTime:     record.EndTime,
				LogPath:     path,
			}
		}
	}

	return status, nil
}

// FindLatestIterationRecord finds the most recent iteration record in the logs directory.
// Returns the record, its path, and any error. Returns nil, "", nil if no records found.
func FindLatestIterationRecord(logsDir string) (*loop.IterationRecord, string, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("failed to read logs directory: %w", err)
	}

	var latestRecord *loop.IterationRecord
	var latestPath string
	var latestEndTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process iteration JSON files
		name := entry.Name()
		if !strings.HasPrefix(name, "iteration-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(logsDir, name)
		record, err := loop.LoadRecord(path)
		if err != nil {
			continue // Skip invalid files
		}

		// Track the most recent by end time
		if latestRecord == nil || record.EndTime.After(latestEndTime) {
			latestRecord = record
			latestPath = path
			latestEndTime = record.EndTime
		}
	}

	return latestRecord, latestPath, nil
}

// FormatStatus formats a status for CLI display.
func FormatStatus(status *Status) string {
	var sb strings.Builder

	sb.WriteString("## Status\n\n")

	// Parent info
	fmt.Fprintf(&sb, "Parent: %s\n\n", status.ParentTaskID)

	// Task counts
	sb.WriteString("### Task Counts\n")
	fmt.Fprintf(&sb, "Total: %d\n", status.Counts.Total)
	fmt.Fprintf(&sb, "Completed: %d\n", status.Counts.Completed)
	fmt.Fprintf(&sb, "Ready: %d\n", status.Counts.Ready)
	fmt.Fprintf(&sb, "Blocked: %d\n", status.Counts.Blocked)
	fmt.Fprintf(&sb, "Failed: %d\n", status.Counts.Failed)
	fmt.Fprintf(&sb, "Skipped: %d\n", status.Counts.Skipped)
	sb.WriteString("\n")

	// Next task
	sb.WriteString("### Next Task\n")
	if status.NextTask != nil {
		fmt.Fprintf(&sb, "Next Task: %s (%s)\n", status.NextTask.ID, status.NextTask.Title)
	} else {
		sb.WriteString("Next Task: none\n")
	}
	sb.WriteString("\n")

	// Last iteration
	if status.LastIteration != nil {
		sb.WriteString("### Last Iteration\n")
		fmt.Fprintf(&sb, "ID: %s\n", status.LastIteration.IterationID)
		fmt.Fprintf(&sb, "Task: %s\n", status.LastIteration.TaskID)
		if status.LastIteration.TaskTitle != "" {
			fmt.Fprintf(&sb, "Title: %s\n", status.LastIteration.TaskTitle)
		}
		fmt.Fprintf(&sb, "Outcome: %s\n", status.LastIteration.Outcome)
		if !status.LastIteration.EndTime.IsZero() {
			fmt.Fprintf(&sb, "Completed: %s\n", status.LastIteration.EndTime.Format(time.RFC3339))
		}
		if status.LastIteration.LogPath != "" {
			fmt.Fprintf(&sb, "Log: %s\n", status.LastIteration.LogPath)
		}
	}

	return sb.String()
}
