package selector

import (
	"github.com/yarlson/go-ralph/internal/taskstore"
)

// ComputeReady computes the ready status for each task in the list.
// A task is ready if all of its dependencies (from dependsOn) are completed.
// Returns a map from task ID to ready status.
func ComputeReady(tasks []*taskstore.Task, graph *Graph) map[string]bool {
	// Build a status lookup map
	statusByID := make(map[string]taskstore.TaskStatus)
	for _, t := range tasks {
		statusByID[t.ID] = t.Status
	}

	ready := make(map[string]bool)
	for _, t := range tasks {
		ready[t.ID] = isTaskReady(t, statusByID, graph)
	}

	return ready
}

// isTaskReady checks if a single task is ready.
// A task is ready if it has no dependencies, or all its dependencies are completed.
func isTaskReady(task *taskstore.Task, statusByID map[string]taskstore.TaskStatus, graph *Graph) bool {
	deps := graph.Dependencies(task.ID)
	if len(deps) == 0 {
		return true
	}

	for _, depID := range deps {
		status, exists := statusByID[depID]
		if !exists {
			// Dependency not found in task list - treat as not ready
			return false
		}
		if status != taskstore.StatusCompleted {
			return false
		}
	}

	return true
}

// IsLeaf returns true if the given task ID has no children (no task has it as parentId).
// A task with no children in the task list is considered a leaf.
func IsLeaf(tasks []*taskstore.Task, taskID string) bool {
	for _, t := range tasks {
		if t.ParentID != nil && *t.ParentID == taskID {
			return false
		}
	}
	return true
}

// GetReadyLeaves returns all tasks that are:
// 1. status = open
// 2. ready = true (all dependencies completed)
// 3. isLeaf = true (no children)
func GetReadyLeaves(tasks []*taskstore.Task, graph *Graph) []*taskstore.Task {
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

		// Filter by isLeaf = true
		if !IsLeaf(tasks, t.ID) {
			continue
		}

		result = append(result, t)
	}

	return result
}
