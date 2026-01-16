package taskstore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLintTask_ValidTask(t *testing.T) {
	task := &Task{
		ID:          "test-1",
		Title:       "Test Task",
		Description: "A test task",
		Status:      StatusOpen,
		Acceptance:  []string{"criteria 1"},
		Verify:      [][]string{{"go", "test"}},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := LintTask(task)
	assert.NoError(t, err)
}

func TestLintTask_EmptyDescription(t *testing.T) {
	task := &Task{
		ID:         "test-1",
		Title:      "Test Task",
		Status:     StatusOpen,
		Acceptance: []string{"criteria 1"},
		Verify:     [][]string{{"go", "test"}},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err := LintTask(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestLintTask_MissingAcceptance(t *testing.T) {
	task := &Task{
		ID:          "test-1",
		Title:       "Test Task",
		Description: "A test task",
		Status:      StatusOpen,
		Verify:      [][]string{{"go", "test"}},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	warnings, err := LintTaskWithWarnings(task)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "acceptance criteria")
}

func TestLintTask_MissingVerifyOnLeaf(t *testing.T) {
	// For this test, we need to pass the context that this is a leaf task
	// We'll test this in LintTaskSet
	task := &Task{
		ID:          "test-1",
		Title:       "Test Task",
		Description: "A test task",
		Status:      StatusOpen,
		Acceptance:  []string{"criteria 1"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// LintTask itself doesn't know if this is a leaf, so it should pass
	err := LintTask(task)
	assert.NoError(t, err)
}

func TestLintTask_InvalidStatus(t *testing.T) {
	task := &Task{
		ID:          "test-1",
		Title:       "Test Task",
		Description: "A test task",
		Status:      TaskStatus("invalid"),
		Acceptance:  []string{"criteria 1"},
		Verify:      [][]string{{"go", "test"}},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := LintTask(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestLintTaskSet_ValidTasks(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "parent",
			Title:       "Parent Task",
			Description: "A parent task",
			Status:      StatusOpen,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "child",
			Title:       "Child Task",
			Description: "A child task",
			Status:      StatusOpen,
			ParentID:    strPtr("parent"),
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestLintTaskSet_MissingDependency(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "task-1",
			Title:       "Task 1",
			Description: "Task 1",
			Status:      StatusOpen,
			DependsOn:   []string{"task-2"}, // task-2 doesn't exist
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "depends on")
	assert.Contains(t, result.Errors[0].Error, "task-2")
}

func TestLintTaskSet_CircularDependency(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "task-1",
			Title:       "Task 1",
			Description: "Task 1",
			Status:      StatusOpen,
			DependsOn:   []string{"task-2"},
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "task-2",
			Title:       "Task 2",
			Description: "Task 2",
			Status:      StatusOpen,
			DependsOn:   []string{"task-1"}, // circular
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "cycle")
}

func TestLintTaskSet_LeafTaskMissingVerify(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "parent",
			Title:       "Parent Task",
			Description: "A parent task",
			Status:      StatusOpen,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "child",
			Title:       "Child Task",
			Description: "A child task",
			Status:      StatusOpen,
			ParentID:    strPtr("parent"),
			Acceptance:  []string{"criteria 1"},
			// Missing Verify - this is a leaf task
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "leaf task")
	assert.Contains(t, result.Errors[0].Error, "verify")
}

func TestLintTaskSet_InvalidParentID(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "child",
			Title:       "Child Task",
			Description: "A child task",
			Status:      StatusOpen,
			ParentID:    strPtr("nonexistent"),
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "parent")
	assert.Contains(t, result.Errors[0].Error, "nonexistent")
}

func TestLintTaskSet_MultipleErrors(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "task-1",
			Title:       "Task 1",
			Description: "", // Empty description
			Status:      StatusOpen,
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "task-2",
			Title:       "Task 2",
			Description: "Task 2",
			Status:      StatusOpen,
			ParentID:    strPtr("nonexistent"), // Invalid parent
			Acceptance:  []string{"criteria 1"},
			Verify:      [][]string{{"go", "test"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 2)
}

func TestLintTaskSet_EmptyTasks(t *testing.T) {
	result := LintTaskSet([]*Task{})
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestLintResult_Error(t *testing.T) {
	result := &LintResult{
		Valid: false,
		Errors: []LintError{
			{TaskID: "task-1", Error: "missing description"},
			{TaskID: "task-2", Error: "invalid status"},
		},
	}

	err := result.Error()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task-1")
	assert.Contains(t, err.Error(), "task-2")
	assert.Contains(t, err.Error(), "2 validation errors")
}

func TestLintResult_Error_NoErrors(t *testing.T) {
	result := &LintResult{
		Valid:  true,
		Errors: []LintError{},
	}

	err := result.Error()
	assert.NoError(t, err)
}

func TestLintError_String(t *testing.T) {
	lintErr := LintError{
		TaskID: "task-1",
		Error:  "missing description",
	}

	assert.Equal(t, "task-1: missing description", lintErr.String())
}

func TestIsLeafTask(t *testing.T) {
	tasks := []*Task{
		{
			ID:        "parent",
			Title:     "Parent",
			Status:    StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "child",
			Title:     "Child",
			ParentID:  strPtr("parent"),
			Status:    StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	assert.False(t, isLeafTask(tasks, "parent"))
	assert.True(t, isLeafTask(tasks, "child"))
	assert.True(t, isLeafTask(tasks, "nonexistent")) // Non-existent tasks are considered leaves
}

func TestLintTaskSet_Warnings(t *testing.T) {
	tasks := []*Task{
		{
			ID:          "task-1",
			Title:       "Task 1",
			Description: "Task 1",
			Status:      StatusOpen,
			// Missing acceptance criteria - should warn
			Verify:    [][]string{{"go", "test"}},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	result := LintTaskSet(tasks)
	assert.True(t, result.Valid) // Warnings don't make it invalid
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0].Warning, "acceptance criteria")
}

// Helper function
func strPtr(s string) *string {
	return &s
}
