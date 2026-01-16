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

func TestReportCmd_Structure(t *testing.T) {
	cmd := newReportCmd()

	assert.Equal(t, "report", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestReportCmd_Flags(t *testing.T) {
	cmd := newReportCmd()

	// --output flag
	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "o", outputFlag.Shorthand)
	assert.Equal(t, "", outputFlag.DefValue)
}

func TestReportCmd_NoParentTaskID(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure but no parent-task-id
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))

	cmd := newReportCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parent-task-id file not found")
}

func TestReportCmd_ParentTaskNotFound(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure with parent-task-id but no task
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("nonexistent"), 0644))

	cmd := newReportCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parent task")
	assert.Contains(t, err.Error(), "not found")
}

func TestReportCmd_DisplaysReport(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

	// Create task store with parent and completed child
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	parentTask := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Test Feature",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(parentTask))

	childTask := &taskstore.Task{
		ID:        "child-task",
		Title:     "Child Task",
		ParentID:  strPtr("parent-task"),
		Status:    taskstore.StatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(childTask))

	// Write parent task ID
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644))

	cmd := newReportCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Feature Report")
	assert.Contains(t, output, "parent-task")
	assert.Contains(t, output, "Test Feature")
	assert.Contains(t, output, "Completed Tasks")
	assert.Contains(t, output, "Child Task")
}

func TestReportCmd_DisplaysBlockedTasks(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

	// Create task store
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	parentTask := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Test Feature",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(parentTask))

	blockedTask := &taskstore.Task{
		ID:        "blocked-task",
		Title:     "Blocked Task",
		ParentID:  strPtr("parent-task"),
		Status:    taskstore.StatusBlocked,
		DependsOn: []string{"missing-dep"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(blockedTask))

	// Write parent task ID
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644))

	cmd := newReportCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Blocked Tasks")
	assert.Contains(t, output, "Blocked Task")
}

func TestReportCmd_DisplaysIterationStats(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create task store
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	parentTask := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Test Feature",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(parentTask))

	// Create iteration record
	record := &loop.IterationRecord{
		IterationID: "iter-001",
		TaskID:      "test-task",
		StartTime:   now.Add(-time.Hour),
		EndTime:     now,
		ClaudeInvocation: loop.ClaudeInvocationMeta{
			TotalCostUSD: 0.25,
		},
		ResultCommit: "abc1234567890",
		Outcome:      loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Write parent task ID
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644))

	cmd := newReportCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Iterations")
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "Total Cost")
	assert.Contains(t, output, "$0.25")
}

func TestReportCmd_OutputToFile(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

	// Create task store
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	parentTask := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Test Feature",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(parentTask))

	completedTask := &taskstore.Task{
		ID:        "completed-task",
		Title:     "Completed Task",
		ParentID:  strPtr("parent-task"),
		Status:    taskstore.StatusCompleted,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(completedTask))

	// Write parent task ID
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644))

	// Output file path
	outputFile := filepath.Join(tmpDir, "report.md")

	cmd := newReportCmd()
	cmd.SetArgs([]string{"--output", outputFile})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify file was created
	require.FileExists(t, outputFile)

	// Read and verify content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Feature Report")
	assert.Contains(t, string(content), "Test Feature")
	assert.Contains(t, string(content), "Completed Task")

	// Verify stdout shows success message
	assert.Contains(t, stdout.String(), outputFile)
}

func TestReportCmd_OutputToFileCreatesDir(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

	// Create task store
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	parentTask := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Test Feature",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(parentTask))

	// Write parent task ID
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644))

	// Output file in non-existent directory
	outputFile := filepath.Join(tmpDir, "reports", "subdir", "report.md")

	cmd := newReportCmd()
	cmd.SetArgs([]string{"--output", outputFile})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify file was created
	require.FileExists(t, outputFile)
}

func TestReportCmd_EmptyReport(t *testing.T) {
	// Create temp directory as workspace
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create .ralph structure
	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "state"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

	// Create task store with just parent (no children)
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)

	now := time.Now()
	parentTask := &taskstore.Task{
		ID:        "parent-task",
		Title:     "Empty Feature",
		Status:    taskstore.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Save(parentTask))

	// Write parent task ID
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644))

	cmd := newReportCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Feature Report")
	assert.Contains(t, output, "No commits")
	assert.Contains(t, output, "No completed tasks")
}
