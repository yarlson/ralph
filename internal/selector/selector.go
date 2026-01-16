package selector

import (
	"sort"

	"github.com/yarlson/go-ralph/internal/taskstore"
)

// SelectNext selects the next ready leaf task from descendants of the given parent.
// It uses the following heuristics:
// 1. Prefer tasks in the same "area" as the last completed task
// 2. Otherwise, use deterministic ordering: createdAt (ascending), then ID (alphabetically)
//
// Parameters:
//   - tasks: all tasks to consider
//   - graph: the dependency graph for these tasks
//   - parentID: the ID of the parent task whose descendants to search
//   - lastCompleted: the most recently completed task (for area preference), can be nil
//
// Returns nil if no ready leaf task is found among descendants.
func SelectNext(tasks []*taskstore.Task, graph *Graph, parentID string, lastCompleted *taskstore.Task) *taskstore.Task {
	if parentID == "" {
		return nil
	}

	// Get all descendants of the parent
	descendants := getDescendants(tasks, parentID)
	if len(descendants) == 0 {
		return nil
	}

	// Build graph from descendants only (to filter properly)
	// We need to use the full graph for dependency checking but filter to descendants
	readyLeaves := getReadyLeavesFromSubset(descendants, graph)
	if len(readyLeaves) == 0 {
		return nil
	}

	// Sort ready leaves by deterministic ordering first
	sortTasksDeterministically(readyLeaves)

	// Apply area preference if lastCompleted has an area label
	lastArea := getArea(lastCompleted)
	if lastArea != "" {
		// Find tasks with matching area
		var matchingArea []*taskstore.Task
		for _, t := range readyLeaves {
			if getArea(t) == lastArea {
				matchingArea = append(matchingArea, t)
			}
		}

		if len(matchingArea) > 0 {
			// Return the first matching-area task (already sorted)
			return matchingArea[0]
		}
	}

	// No area preference or no matches - use deterministic ordering
	return readyLeaves[0]
}

// getDescendants returns all tasks that are descendants of the given parent.
// A descendant is any task that has the parent as an ancestor (direct or indirect parent).
func getDescendants(tasks []*taskstore.Task, parentID string) []*taskstore.Task {
	// Build parent-to-children map
	children := make(map[string][]*taskstore.Task)
	for _, t := range tasks {
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t)
		}
	}

	// BFS to find all descendants
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

// getReadyLeavesFromSubset returns ready leaf tasks from the given subset of tasks.
// Uses the provided graph for dependency checking.
func getReadyLeavesFromSubset(tasks []*taskstore.Task, graph *Graph) []*taskstore.Task {
	// Build status lookup map for ready computation
	statusByID := make(map[string]taskstore.TaskStatus)
	for _, t := range tasks {
		statusByID[t.ID] = t.Status
	}

	// We also need to check statuses from the full graph for dependencies outside subset
	// However, ComputeReady already handles this by checking graph.Dependencies

	ready := ComputeReady(tasks, graph)

	var result []*taskstore.Task
	for _, t := range tasks {
		// Filter by status = open
		if t.Status != taskstore.StatusOpen {
			continue
		}

		// Filter by ready = true
		if !ready[t.ID] {
			continue
		}

		// Filter by isLeaf = true (within this subset)
		if !IsLeaf(tasks, t.ID) {
			continue
		}

		result = append(result, t)
	}

	return result
}

// sortTasksDeterministically sorts tasks by createdAt (ascending), then ID (alphabetically).
func sortTasksDeterministically(tasks []*taskstore.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		// First compare by createdAt
		if !tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		}
		// Then compare by ID
		return tasks[i].ID < tasks[j].ID
	})
}

// getArea returns the "area" label value from a task, or empty string if not present.
func getArea(task *taskstore.Task) string {
	if task == nil || task.Labels == nil {
		return ""
	}
	return task.Labels["area"]
}
