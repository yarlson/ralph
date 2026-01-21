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

func TestFixCommand_Structure(t *testing.T) {
	cmd := newFixCmd()

	assert.Equal(t, "fix", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	// Check for flags
	assert.NotNil(t, cmd.Flags().Lookup("list"))
	assert.NotNil(t, cmd.Flags().Lookup("retry"))
	assert.NotNil(t, cmd.Flags().Lookup("skip"))
	assert.NotNil(t, cmd.Flags().Lookup("undo"))
}

func TestFixCommand_ListEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"fix", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Failed Tasks")
	assert.Contains(t, output, "none")
}

func TestFixCommand_RetryFailedTask(t *testing.T) {
	tmpDir := t.TempDir()

	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	task := &taskstore.Task{
		ID:        "task-failed-1",
		Title:     "A Failed Task",
		Status:    taskstore.StatusFailed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, store.Save(task))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte("tasks:\n  path: \".ralph/tasks\"\n"), 0644))

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"fix", "--retry", "task-failed-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	updated, err := store.Get("task-failed-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)
}

func TestFixCommand_SkipTask(t *testing.T) {
	tmpDir := t.TempDir()

	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	task := &taskstore.Task{
		ID:        "task-open-1",
		Title:     "An Open Task",
		Status:    taskstore.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, store.Save(task))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte("tasks:\n  path: \".ralph/tasks\"\n"), 0644))

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"fix", "--skip", "task-open-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	updated, err := store.Get("task-open-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}

func TestFixCommand_UndoIterationNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte("tasks:\n  path: \".ralph/tasks\"\n"), 0644))

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"fix", "--undo", "nonexistent", "--force"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration not found")
}
