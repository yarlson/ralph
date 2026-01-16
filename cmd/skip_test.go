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

func TestSkipCommand_Structure(t *testing.T) {
	cmd := newSkipCmd()

	assert.Equal(t, "skip", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check for --task flag
	taskFlag := cmd.Flags().Lookup("task")
	require.NotNil(t, taskFlag, "should have --task flag")
	assert.Equal(t, "", taskFlag.DefValue)

	// Check for --reason flag
	reasonFlag := cmd.Flags().Lookup("reason")
	require.NotNil(t, reasonFlag, "should have --reason flag")
	assert.Equal(t, "", reasonFlag.DefValue)
}

func TestSkipCommand_MissingTaskFlag(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"skip"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--task")
}

func TestSkipCommand_TaskNotFound(t *testing.T) {
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
	cmd.SetArgs([]string{"skip", "--task", "nonexistent"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSkipCommand_SkipOpenTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create an open task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    taskstore.StatusOpen,
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
	cmd.SetArgs([]string{"skip", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)

	// Check output
	assert.Contains(t, out.String(), "task-1")
	assert.Contains(t, out.String(), "skipped")
}

func TestSkipCommand_SkipFailedTask(t *testing.T) {
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
		Title:     "Failed Task",
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
	cmd.SetArgs([]string{"skip", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}

func TestSkipCommand_SkipBlockedTask(t *testing.T) {
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
	cmd.SetArgs([]string{"skip", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}

func TestSkipCommand_CannotSkipCompleted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create a completed task
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
	cmd.SetArgs([]string{"skip", "--task", "task-1"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot skip")
}

func TestSkipCommand_AlreadySkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create a skipped task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Skipped Task",
		Status:    taskstore.StatusSkipped,
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
	cmd.SetArgs([]string{"skip", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err) // Should succeed but indicate already skipped

	// Check output indicates already skipped
	assert.Contains(t, out.String(), "already skipped")
}

func TestSkipCommand_WithReason(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	stateDir := filepath.Join(tmpDir, ".ralph", "state")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(stateDir, 0755))

	// Create an open task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    taskstore.StatusOpen,
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
	cmd.SetArgs([]string{"skip", "--task", "task-1", "--reason", "Not needed for MVP"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Check that reason file was created
	reasonFile := filepath.Join(stateDir, "skip-reason-task-1.txt")
	content, err := os.ReadFile(reasonFile)
	require.NoError(t, err)
	assert.Equal(t, "Not needed for MVP", string(content))
}

func TestSkipCommand_SkipInProgressTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create an in_progress task - can be skipped
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
	cmd.SetArgs([]string{"skip", "--task", "task-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}
