package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/taskstore"
)

func TestFixCommand_Structure(t *testing.T) {
	cmd := newFixCmd()

	assert.Equal(t, "fix", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check for --list flag
	listFlag := cmd.Flags().Lookup("list")
	require.NotNil(t, listFlag, "should have --list flag")
	assert.Equal(t, "false", listFlag.DefValue)

	// Check shorthand
	listFlagShort := cmd.Flags().ShorthandLookup("l")
	require.NotNil(t, listFlagShort, "should have -l shorthand")

	// Check for --retry flag
	retryFlag := cmd.Flags().Lookup("retry")
	require.NotNil(t, retryFlag, "should have --retry flag")

	// Check for --skip flag
	skipFlag := cmd.Flags().Lookup("skip")
	require.NotNil(t, skipFlag, "should have --skip flag")

	// Check for --undo flag
	undoFlag := cmd.Flags().Lookup("undo")
	require.NotNil(t, undoFlag, "should have --undo flag")
}

func TestFixCommand_ListEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

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
	cmd.SetArgs([]string{"fix", "--list"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Failed Tasks")
	assert.Contains(t, output, "Blocked Tasks")
	assert.Contains(t, output, "Recent Iterations")
	assert.Contains(t, output, "none")
}

func TestFixCommand_ListFailedTasks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create a failed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-failed-1",
		Title:     "A Failed Task",
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
	cmd.SetArgs([]string{"fix", "--list"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Failed Tasks")
	assert.Contains(t, output, "task-failed-1")
	assert.Contains(t, output, "A Failed Task")
}

func TestFixCommand_ListBlockedTasks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create a blocked task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-blocked-1",
		Title:     "A Blocked Task",
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
	cmd.SetArgs([]string{"fix", "--list"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Blocked Tasks")
	assert.Contains(t, output, "task-blocked-1")
	assert.Contains(t, output, "A Blocked Task")
}

func TestFixCommand_ListRecentIterations(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an iteration record
	record := &loop.IterationRecord{
		IterationID: "abc12345",
		TaskID:      "task-1",
		StartTime:   time.Now().Add(-10 * time.Minute),
		EndTime:     time.Now().Add(-5 * time.Minute),
		Outcome:     loop.OutcomeFailed,
	}
	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

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
	cmd.SetArgs([]string{"fix", "--list"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Recent Iterations")
	assert.Contains(t, output, "abc12345")
	assert.Contains(t, output, "task-1")
	assert.Contains(t, output, "failed")
}

func TestFixCommand_ListMultipleItems(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create task store
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()

	// Create multiple failed and blocked tasks
	failedTask1 := &taskstore.Task{
		ID:        "task-failed-1",
		Title:     "First Failed Task",
		Status:    taskstore.StatusFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	failedTask2 := &taskstore.Task{
		ID:        "task-failed-2",
		Title:     "Second Failed Task",
		Status:    taskstore.StatusFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	blockedTask := &taskstore.Task{
		ID:        "task-blocked-1",
		Title:     "Blocked Task",
		Status:    taskstore.StatusBlocked,
		CreatedAt: now,
		UpdatedAt: now,
	}
	completedTask := &taskstore.Task{
		ID:        "task-completed-1",
		Title:     "Completed Task",
		Status:    taskstore.StatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, store.Save(failedTask1))
	require.NoError(t, store.Save(failedTask2))
	require.NoError(t, store.Save(blockedTask))
	require.NoError(t, store.Save(completedTask))

	// Create iteration records
	record1 := &loop.IterationRecord{
		IterationID: "iter0001",
		TaskID:      "task-failed-1",
		StartTime:   now.Add(-20 * time.Minute),
		EndTime:     now.Add(-15 * time.Minute),
		Outcome:     loop.OutcomeFailed,
	}
	record2 := &loop.IterationRecord{
		IterationID: "iter0002",
		TaskID:      "task-completed-1",
		StartTime:   now.Add(-10 * time.Minute),
		EndTime:     now.Add(-5 * time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record1)
	require.NoError(t, err)
	_, err = loop.SaveRecord(logsDir, record2)
	require.NoError(t, err)

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
	cmd.SetArgs([]string{"fix", "--list"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Check failed tasks are listed
	assert.Contains(t, output, "task-failed-1")
	assert.Contains(t, output, "task-failed-2")

	// Check blocked task is listed
	assert.Contains(t, output, "task-blocked-1")

	// Check completed task is NOT listed as failed or blocked
	// but iteration is listed
	assert.Contains(t, output, "iter0001")
	assert.Contains(t, output, "iter0002")
}

func TestFixCommand_ListShorthand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

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
	cmd.SetArgs([]string{"fix", "-l"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Failed Tasks")
	assert.Contains(t, output, "Blocked Tasks")
	assert.Contains(t, output, "Recent Iterations")
}
