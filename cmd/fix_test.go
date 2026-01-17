package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestFixCommand_NonTTY_NoFlags_ShowsError(t *testing.T) {
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
	failedTask := &taskstore.Task{
		ID:        "task-failed-1",
		Title:     "A Failed Task",
		Status:    taskstore.StatusFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(failedTask))

	// Create an iteration record
	record := &loop.IterationRecord{
		IterationID: "abc12345",
		TaskID:      "task-1",
		StartTime:   now.Add(-10 * time.Minute),
		EndTime:     now.Add(-5 * time.Minute),
		Outcome:     loop.OutcomeFailed,
	}
	_, err = loop.SaveRecord(logsDir, record)
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
	cmd.SetArgs([]string{"fix"})

	// Simulate non-TTY by providing a non-TTY stdin via file
	// Create a temp file to use as stdin (non-TTY)
	tmpFile, err := os.CreateTemp("", "non-tty-*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()
	cmd.SetIn(tmpFile)

	err = cmd.Execute()
	require.Error(t, err)

	// Check error message
	assert.Contains(t, err.Error(), "interactive mode requires TTY")

	// Check stderr/stdout contains guidance
	output := out.String()
	assert.Contains(t, output, "Fixable Issues:")
	assert.Contains(t, output, "task-failed-1")
	assert.Contains(t, output, "ralph fix --retry")
	assert.Contains(t, output, "Recent Iterations:")
	assert.Contains(t, output, "abc12345")
	assert.Contains(t, output, "ralph fix --undo")
}

func TestFixCommand_NonTTY_WithFlag_Works(t *testing.T) {
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

	// Simulate non-TTY by providing a non-TTY stdin via file
	tmpFile, err := os.CreateTemp("", "non-tty-*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()
	cmd.SetIn(tmpFile)

	err = cmd.Execute()
	require.NoError(t, err)

	// With --list flag, should work even in non-TTY
	output := out.String()
	assert.Contains(t, output, "Failed Tasks")
}

func TestFixCommand_RetryFailedTask(t *testing.T) {
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
	cmd.SetArgs([]string{"fix", "--retry", "task-failed-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was reset
	updated, err := store.Get("task-failed-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)

	// Check output confirms retry initiated
	output := out.String()
	assert.Contains(t, output, "task-failed-1")
	assert.Contains(t, output, "Retry initiated")
}

func TestFixCommand_RetryOpenTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an open task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-open-1",
		Title:     "An Open Task",
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
	cmd.SetArgs([]string{"fix", "--retry", "task-open-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Task should remain open
	updated, err := store.Get("task-open-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)

	// Check output indicates already open
	output := out.String()
	assert.Contains(t, output, "already open")
}

func TestFixCommand_RetryCompletedTaskError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create a completed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-completed-1",
		Title:     "A Completed Task",
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
	cmd.SetArgs([]string{"fix", "--retry", "task-completed-1"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot retry")
	assert.Contains(t, err.Error(), "completed")
}

func TestFixCommand_RetryInProgressTaskError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an in_progress task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-inprogress-1",
		Title:     "An In Progress Task",
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
	cmd.SetArgs([]string{"fix", "--retry", "task-inprogress-1"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot retry")
	assert.Contains(t, err.Error(), "must be failed or open")
}

func TestFixCommand_RetryWithFeedback(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	stateDir := filepath.Join(tmpDir, ".ralph", "state")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(stateDir, 0755))
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
	cmd.SetArgs([]string{"fix", "--retry", "task-failed-1", "--feedback", "Try using a different algorithm"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Check that feedback file was created
	feedbackFile := filepath.Join(stateDir, "feedback-task-failed-1.txt")
	content, err := os.ReadFile(feedbackFile)
	require.NoError(t, err)
	assert.Equal(t, "Try using a different algorithm", string(content))

	// Check output mentions feedback
	output := out.String()
	assert.Contains(t, output, "Feedback saved")
}

func TestFixCommand_RetryTaskNotFound(t *testing.T) {
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
	cmd.SetArgs([]string{"fix", "--retry", "nonexistent-task"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFixCommand_RetryShorthand(t *testing.T) {
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
	// Use shorthand -r and -f
	cmd.SetArgs([]string{"fix", "-r", "task-failed-1", "-f", "Use shorthand flags"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was reset
	updated, err := store.Get("task-failed-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)
}

func TestFixCommand_SkipOpenTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an open task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-open-1",
		Title:     "An Open Task",
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
	cmd.SetArgs([]string{"fix", "--skip", "task-open-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-open-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)

	// Check output confirms task skipped
	output := out.String()
	assert.Contains(t, output, "task-open-1")
	assert.Contains(t, output, "skipped")
}

func TestFixCommand_SkipFailedTask(t *testing.T) {
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
	cmd.SetArgs([]string{"fix", "--skip", "task-failed-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-failed-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}

func TestFixCommand_SkipBlockedTask(t *testing.T) {
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
	cmd.SetArgs([]string{"fix", "--skip", "task-blocked-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-blocked-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}

func TestFixCommand_SkipCompletedTaskError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create a completed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-completed-1",
		Title:     "A Completed Task",
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
	cmd.SetArgs([]string{"fix", "--skip", "task-completed-1"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot skip completed task")
}

func TestFixCommand_SkipWithReason(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	stateDir := filepath.Join(tmpDir, ".ralph", "state")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an open task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-open-1",
		Title:     "An Open Task",
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
	cmd.SetArgs([]string{"fix", "--skip", "task-open-1", "--reason", "Not needed for MVP"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Check that reason file was created
	reasonFile := filepath.Join(stateDir, "skip-reason-task-open-1.txt")
	content, err := os.ReadFile(reasonFile)
	require.NoError(t, err)
	assert.Equal(t, "Not needed for MVP", string(content))
}

func TestFixCommand_SkipTaskNotFound(t *testing.T) {
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
	cmd.SetArgs([]string{"fix", "--skip", "nonexistent-task"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFixCommand_SkipShorthand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an open task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-open-1",
		Title:     "An Open Task",
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
	// Use shorthand -s
	cmd.SetArgs([]string{"fix", "-s", "task-open-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-open-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)
}

func TestFixCommand_SkipAlreadySkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create a skipped task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-skipped-1",
		Title:     "A Skipped Task",
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
	cmd.SetArgs([]string{"fix", "--skip", "task-skipped-1"})

	err = cmd.Execute()
	require.NoError(t, err) // Should succeed but indicate already skipped

	// Check output indicates already skipped
	assert.Contains(t, out.String(), "already skipped")
}

func TestFixCommand_SkipInProgressTaskError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create an in_progress task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	task := &taskstore.Task{
		ID:        "task-inprogress-1",
		Title:     "An In Progress Task",
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
	cmd.SetArgs([]string{"fix", "--skip", "task-inprogress-1"})

	err = cmd.Execute()
	// in_progress tasks cannot be skipped per acceptance criteria (only open, failed, blocked)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot skip")
}

func TestFixCommand_UndoIterationNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
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
	cmd.SetArgs([]string{"fix", "--undo", "nonexistent", "--force"})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration not found")
}

func TestFixCommand_UndoWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	require.NoError(t, runGitCommand(tmpDir, "init", "-b", "main"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.email", "test@test.com"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitCommand(tmpDir, "config", "commit.gpgsign", "false"))

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "initial"))

	// Get base commit
	baseCommit := getGitCommit(t, tmpDir)

	// Make a change
	require.NoError(t, os.WriteFile(testFile, []byte("changed"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "iteration change"))

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create task store with a completed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)
	now := time.Now()
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test task for undo",
		Status:      taskstore.StatusCompleted,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, store.Save(task))

	// Create iteration record
	record := &loop.IterationRecord{
		IterationID:  "test123",
		TaskID:       "test-task",
		BaseCommit:   baseCommit,
		Outcome:      loop.OutcomeSuccess,
		FilesChanged: []string{"test.txt"},
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"fix", "--undo", "test123", "--force"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify we're back at base commit
	currentCommit := getGitCommit(t, tmpDir)
	assert.Equal(t, baseCommit, currentCommit)

	// Verify task status was updated
	updatedTask, err := store.Get("test-task")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updatedTask.Status)

	// Verify output
	output := out.String()
	assert.Contains(t, output, "Undo completed")
	assert.Contains(t, output, "test123")
}

func TestFixCommand_UndoShorthand(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	require.NoError(t, runGitCommand(tmpDir, "init", "-b", "main"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.email", "test@test.com"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitCommand(tmpDir, "config", "commit.gpgsign", "false"))

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "initial"))

	baseCommit := getGitCommit(t, tmpDir)

	// Make a change
	require.NoError(t, os.WriteFile(testFile, []byte("changed"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "iteration change"))

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create iteration record
	record := &loop.IterationRecord{
		IterationID: "abc123",
		TaskID:      "task-1",
		BaseCommit:  baseCommit,
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Use shorthand -u
	cmd.SetArgs([]string{"fix", "-u", "abc123", "--force"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify we're back at base commit
	currentCommit := getGitCommit(t, tmpDir)
	assert.Equal(t, baseCommit, currentCommit)
}

func TestFixCommand_UndoShowsConfirmation(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	require.NoError(t, runGitCommand(tmpDir, "init", "-b", "main"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.email", "test@test.com"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitCommand(tmpDir, "config", "commit.gpgsign", "false"))

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "initial"))

	baseCommit := getGitCommit(t, tmpDir)

	// Make a change
	require.NoError(t, os.WriteFile(testFile, []byte("changed"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "iteration change"))

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create task store with a completed task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)
	now := time.Now()
	task := &taskstore.Task{
		ID:        "test-task",
		Title:     "Test Task",
		Status:    taskstore.StatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(task))

	// Create iteration record with files changed
	record := &loop.IterationRecord{
		IterationID:  "conf123",
		TaskID:       "test-task",
		BaseCommit:   baseCommit,
		Outcome:      loop.OutcomeSuccess,
		FilesChanged: []string{"test.txt", "another.txt"},
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide "no" response via stdin to cancel
	cmd.SetIn(bytes.NewReader([]byte("no\n")))
	cmd.SetArgs([]string{"fix", "--undo", "conf123"})

	err = cmd.Execute()
	require.NoError(t, err) // Cancelled is not an error

	// Verify confirmation shows expected info
	output := out.String()
	assert.Contains(t, output, "Commit to reset to:")
	assert.Contains(t, output, baseCommit[:7]) // Short hash
	assert.Contains(t, output, "Task to reopen:")
	assert.Contains(t, output, "test-task")
	assert.Contains(t, output, "Files to revert:")
	assert.Contains(t, output, "test.txt")
	assert.Contains(t, output, "another.txt")
	assert.Contains(t, output, "cancelled")
}

func TestFixCommand_UndoWarnsUncommittedChanges(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	require.NoError(t, runGitCommand(tmpDir, "init", "-b", "main"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.email", "test@test.com"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitCommand(tmpDir, "config", "commit.gpgsign", "false"))

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "initial"))

	baseCommit := getGitCommit(t, tmpDir)

	// Make a committed change
	require.NoError(t, os.WriteFile(testFile, []byte("changed"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "iteration change"))

	// Make an uncommitted change
	require.NoError(t, os.WriteFile(testFile, []byte("uncommitted"), 0644))

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create iteration record
	record := &loop.IterationRecord{
		IterationID: "warn123",
		TaskID:      "task-1",
		BaseCommit:  baseCommit,
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide "no" response via stdin to cancel
	cmd.SetIn(bytes.NewReader([]byte("no\n")))
	cmd.SetArgs([]string{"fix", "--undo", "warn123"})

	err = cmd.Execute()
	require.NoError(t, err) // Cancelled is not an error

	// Verify warning about uncommitted changes is shown
	output := out.String()
	assert.Contains(t, output, "WARNING")
	assert.Contains(t, output, "uncommitted changes")
}

func TestFixCommand_UndoConfirmYes(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	require.NoError(t, runGitCommand(tmpDir, "init", "-b", "main"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.email", "test@test.com"))
	require.NoError(t, runGitCommand(tmpDir, "config", "user.name", "Test User"))
	require.NoError(t, runGitCommand(tmpDir, "config", "commit.gpgsign", "false"))

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "initial"))

	baseCommit := getGitCommit(t, tmpDir)

	// Make a change
	require.NoError(t, os.WriteFile(testFile, []byte("changed"), 0644))
	require.NoError(t, runGitCommand(tmpDir, "add", "."))
	require.NoError(t, runGitCommand(tmpDir, "commit", "-m", "iteration change"))

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create iteration record
	record := &loop.IterationRecord{
		IterationID: "yes123",
		TaskID:      "task-1",
		BaseCommit:  baseCommit,
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide "yes" response via stdin
	cmd.SetIn(bytes.NewReader([]byte("yes\n")))
	cmd.SetArgs([]string{"fix", "--undo", "yes123"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify we're back at base commit
	currentCommit := getGitCommit(t, tmpDir)
	assert.Equal(t, baseCommit, currentCommit)

	// Verify completion message
	output := out.String()
	assert.Contains(t, output, "Undo completed")
}

// Helper function to run git commands in tests
func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// Helper function to get current git commit in tests
func getGitCommit(t *testing.T, dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(output))
}

// Interactive mode tests - Note: These use a simulated TTY via SetTTYForTesting

func TestFixCommand_InteractiveMode_ShowsIssues(t *testing.T) {
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

	// Create an iteration record
	record := &loop.IterationRecord{
		IterationID: "abc12345",
		TaskID:      "task-1",
		StartTime:   now.Add(-10 * time.Minute),
		EndTime:     now.Add(-5 * time.Minute),
		Outcome:     loop.OutcomeFailed,
	}
	_, err = loop.SaveRecord(logsDir, record)
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

	// Force TTY mode for testing
	SetTTYForTesting(true)
	defer SetTTYForTesting(false)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide 'q' to quit interactive mode
	cmd.SetIn(bytes.NewReader([]byte("q\n")))
	cmd.SetArgs([]string{"fix"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Should show issues
	assert.Contains(t, output, "task-failed-1")
	assert.Contains(t, output, "A Failed Task")
	// Should show iterations
	assert.Contains(t, output, "abc12345")
	// Should show commands help
	assert.Contains(t, output, "r <id>")
	assert.Contains(t, output, "s <id>")
	assert.Contains(t, output, "u <id>")
	assert.Contains(t, output, "rf <id>")
	assert.Contains(t, output, "q")
}

func TestFixCommand_InteractiveMode_RetryCommand(t *testing.T) {
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

	// Force TTY mode for testing
	SetTTYForTesting(true)
	defer SetTTYForTesting(false)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide 'r task-failed-1' then 'q' to quit
	cmd.SetIn(bytes.NewReader([]byte("r task-failed-1\nq\n")))
	cmd.SetArgs([]string{"fix"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was reset
	updated, err := store.Get("task-failed-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updated.Status)

	// Verify output shows retry success
	output := out.String()
	assert.Contains(t, output, "Retry initiated")
}

func TestFixCommand_InteractiveMode_SkipCommand(t *testing.T) {
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

	// Force TTY mode for testing
	SetTTYForTesting(true)
	defer SetTTYForTesting(false)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide 's task-failed-1' then 'q' to quit
	cmd.SetIn(bytes.NewReader([]byte("s task-failed-1\nq\n")))
	cmd.SetArgs([]string{"fix"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify task status was set to skipped
	updated, err := store.Get("task-failed-1")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusSkipped, updated.Status)

	// Verify output shows skip success
	output := out.String()
	assert.Contains(t, output, "skipped")
}

func TestFixCommand_InteractiveMode_QuitCommand(t *testing.T) {
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

	// Force TTY mode for testing
	SetTTYForTesting(true)
	defer SetTTYForTesting(false)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide 'q' to quit immediately
	cmd.SetIn(bytes.NewReader([]byte("q\n")))
	cmd.SetArgs([]string{"fix"})

	err = cmd.Execute()
	require.NoError(t, err) // Should exit cleanly
}

func TestFixCommand_InteractiveMode_ShowsAttemptCounts(t *testing.T) {
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

	// Create multiple iteration records for the same task
	for i := 0; i < 3; i++ {
		record := &loop.IterationRecord{
			IterationID: fmt.Sprintf("iter%d", i),
			TaskID:      "task-failed-1",
			StartTime:   now.Add(-time.Duration(10+i) * time.Minute),
			EndTime:     now.Add(-time.Duration(5+i) * time.Minute),
			Outcome:     loop.OutcomeFailed,
		}
		_, err = loop.SaveRecord(logsDir, record)
		require.NoError(t, err)
	}

	// Write ralph.yaml
	configContent := `tasks:
  path: ".ralph/tasks"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte(configContent), 0644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Force TTY mode for testing
	SetTTYForTesting(true)
	defer SetTTYForTesting(false)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide 'q' to quit interactive mode
	cmd.SetIn(bytes.NewReader([]byte("q\n")))
	cmd.SetArgs([]string{"fix"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Should show attempt count of 3
	assert.Contains(t, output, "attempts: 3")
}
