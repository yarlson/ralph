package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/taskstore"
)

func TestRetryCommand_Structure(t *testing.T) {
	cmd := newRetryCmd()

	assert.Equal(t, "retry", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check for --task flag
	taskFlag := cmd.Flags().Lookup("task")
	require.NotNil(t, taskFlag, "should have --task flag")
	assert.Equal(t, "", taskFlag.DefValue)

	// Check for --feedback flag
	feedbackFlag := cmd.Flags().Lookup("feedback")
	require.NotNil(t, feedbackFlag, "should have --feedback flag")
	assert.Equal(t, "", feedbackFlag.DefValue)
}

func TestRetryCommand_MissingTaskFlag(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--task")
}

func TestRetryCommand_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry", "--task", "nonexistent"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRetryCommand_ResetFailedTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create a failed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    taskstore.StatusFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(task))

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was reset
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)

	// Check output
	assert.Contains(t, out.String(), "task-1")
	assert.Contains(t, out.String(), "open")
}

func TestRetryCommand_ResetBlockedTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create a blocked task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Blocked Task",
		Status:    taskstore.StatusBlocked,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(task))

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was reset
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)
}

func TestRetryCommand_InvalidStateCompleted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create a completed task - shouldn't be retried
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Completed Task",
		Status:    taskstore.StatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(task))

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry", "--task", "task-1"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot retry")
}

func TestRetryCommand_WithFeedback(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	feedbackDir := filepath.Join(tmpDir, ".ralph", "state")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(feedbackDir, 0755))

	// Create a failed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    taskstore.StatusFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(task))

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry", "--task", "task-1", "--feedback", "Try a different approach"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Check that feedback file was created
	feedbackFile := filepath.Join(feedbackDir, "feedback-task-1.txt")
	content, err := os.ReadFile(feedbackFile)
	require.NoError(t, err)
	assert.Equal(t, "Try a different approach", string(content))
}

func TestRetryCommand_ResetInProgressTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create an in_progress task - can be retried
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "In Progress Task",
		Status:    taskstore.StatusInProgress,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(task))

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"retry", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was reset
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)
}
