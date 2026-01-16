package taskstore

import (
	"fmt"
	"sort"
	"strings"
)

// LintError represents a validation error for a specific task.
type LintError struct {
	TaskID string
	Error  string
}

// String returns a formatted string representation of the lint error.
func (e LintError) String() string {
	return fmt.Sprintf("%s: %s", e.TaskID, e.Error)
}

// LintWarning represents a non-fatal validation warning for a specific task.
type LintWarning struct {
	TaskID  string
	Warning string
}

// String returns a formatted string representation of the lint warning.
func (w LintWarning) String() string {
	return fmt.Sprintf("%s: %s", w.TaskID, w.Warning)
}

// LintResult contains the results of linting a task or task set.
type LintResult struct {
	Valid    bool
	Errors   []LintError
	Warnings []LintWarning
}

// Error returns an error if the lint result is invalid, or nil if valid.
func (r *LintResult) Error() error {
	if r.Valid {
		return nil
	}
	if len(r.Errors) == 0 {
		return nil
	}

	var errMsgs []string
	for _, lintErr := range r.Errors {
		errMsgs = append(errMsgs, lintErr.String())
	}

	return fmt.Errorf("%d validation errors:\n%s", len(r.Errors), strings.Join(errMsgs, "\n"))
}

// LintTask validates a single task.
// Returns an error if the task is invalid.
func LintTask(task *Task) error {
	warnings, err := LintTaskWithWarnings(task)
	_ = warnings // Ignore warnings in strict mode
	return err
}

// LintTaskWithWarnings validates a single task and returns both errors and warnings.
// Warnings are non-fatal issues that don't prevent the task from being used.
func LintTaskWithWarnings(task *Task) ([]string, error) {
	var warnings []string

	// First check basic validation
	if err := task.Validate(); err != nil {
		return warnings, err
	}

	// Check description is non-empty
	if strings.TrimSpace(task.Description) == "" {
		return warnings, fmt.Errorf("task description is required")
	}

	// Warn if acceptance criteria is missing (non-fatal)
	if len(task.Acceptance) == 0 {
		warnings = append(warnings, "acceptance criteria missing (recommended for verification)")
	}

	return warnings, nil
}

// LintTaskSet validates an entire set of tasks.
// It checks for:
// - Individual task validity
// - Dependency existence
// - Dependency cycles
// - Parent ID validity
// - Leaf tasks have verify commands
func LintTaskSet(tasks []*Task) *LintResult {
	result := &LintResult{
		Valid:    true,
		Errors:   []LintError{},
		Warnings: []LintWarning{},
	}

	if len(tasks) == 0 {
		return result
	}

	// Build task ID lookup map
	taskMap := make(map[string]*Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Validate each task individually
	for _, task := range tasks {
		warnings, err := LintTaskWithWarnings(task)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, LintError{
				TaskID: task.ID,
				Error:  err.Error(),
			})
		}

		// Collect warnings
		for _, warning := range warnings {
			result.Warnings = append(result.Warnings, LintWarning{
				TaskID:  task.ID,
				Warning: warning,
			})
		}
	}

	// Validate parent IDs
	for _, task := range tasks {
		if task.ParentID != nil && *task.ParentID != "" {
			if _, exists := taskMap[*task.ParentID]; !exists {
				result.Valid = false
				result.Errors = append(result.Errors, LintError{
					TaskID: task.ID,
					Error:  fmt.Sprintf("parent task %q does not exist", *task.ParentID),
				})
			}
		}
	}

	// Validate dependencies exist
	for _, task := range tasks {
		for _, depID := range task.DependsOn {
			if _, exists := taskMap[depID]; !exists {
				result.Valid = false
				result.Errors = append(result.Errors, LintError{
					TaskID: task.ID,
					Error:  fmt.Sprintf("depends on task %q which does not exist", depID),
				})
			}
		}
	}

	// Check for dependency cycles
	if cycle := detectDependencyCycle(tasks); cycle != nil {
		result.Valid = false
		result.Errors = append(result.Errors, LintError{
			TaskID: "",
			Error:  fmt.Sprintf("dependency cycle detected: %v", cycle),
		})
	}

	// Check leaf tasks have verify commands
	for _, task := range tasks {
		if isLeafTask(tasks, task.ID) {
			if len(task.Verify) == 0 {
				result.Valid = false
				result.Errors = append(result.Errors, LintError{
					TaskID: task.ID,
					Error:  "leaf task must have verify commands",
				})
			}
		}
	}

	return result
}

// isLeafTask returns true if the given task is a leaf task (no children).
func isLeafTask(tasks []*Task, taskID string) bool {
	for _, task := range tasks {
		if task.ParentID != nil && *task.ParentID == taskID {
			return false
		}
	}
	return true
}

// detectDependencyCycle checks if there is a cycle in the dependency graph.
// Returns the cycle path as a slice of task IDs if a cycle is found, or nil if no cycle exists.
// Uses depth-first search with coloring (white=unvisited, gray=in-progress, black=done).
func detectDependencyCycle(tasks []*Task) []string {
	const (
		white = 0 // unvisited
		gray  = 1 // in current DFS path
		black = 2 // fully explored
	)

	// Build edges map
	edges := make(map[string][]string)
	for _, task := range tasks {
		edges[task.ID] = task.DependsOn
	}

	color := make(map[string]int)
	parent := make(map[string]string)

	// Get sorted node IDs for deterministic traversal
	var nodeIDs []string
	for _, task := range tasks {
		nodeIDs = append(nodeIDs, task.ID)
	}
	sort.Strings(nodeIDs)

	var dfs func(node string) []string
	dfs = func(node string) []string {
		color[node] = gray

		for _, dep := range edges[node] {
			if color[dep] == gray {
				// Found a cycle - reconstruct path
				cycle := []string{dep, node}
				for curr := node; curr != dep && parent[curr] != ""; curr = parent[curr] {
					if curr != node {
						cycle = append(cycle, curr)
					}
				}
				return cycle
			}
			if color[dep] == white {
				parent[dep] = node
				if cyclePath := dfs(dep); cyclePath != nil {
					return cyclePath
				}
			}
		}

		color[node] = black
		return nil
	}

	for _, node := range nodeIDs {
		if color[node] == white {
			if cyclePath := dfs(node); cyclePath != nil {
				return cyclePath
			}
		}
	}

	return nil
}
