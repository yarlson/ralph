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
	assert.Contains(t, err.Error(), "Cannot skip completed task")
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
