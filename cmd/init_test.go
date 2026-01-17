package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/state"
	"github.com/yarlson/ralph/internal/taskstore"
)

// Helper to create a valid parent task (not a leaf)
func createValidParentTask(id, title string) *taskstore.Task {
	return &taskstore.Task{
		ID:          id,
		Title:       title,
		Description: title + " description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// Helper to create a valid leaf task
func createValidLeafTask(id, title string, parentID *string) *taskstore.Task {
	return &taskstore.Task{
		ID:          id,
		Title:       title,
		Description: title + " description",
		Status:      taskstore.StatusOpen,
		ParentID:    parentID,
		Verify:      [][]string{{"go", "test"}},
		Acceptance:  []string{"Test passes"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestInitCommand(t *testing.T) {
	t.Run("has --parent flag", func(t *testing.T) {
		cmd := newInitCmd()
		flag := cmd.Flags().Lookup("parent")
		require.NotNil(t, flag, "expected --parent flag to exist")
	})

	t.Run("has --search flag", func(t *testing.T) {
		cmd := newInitCmd()
		flag := cmd.Flags().Lookup("search")
		require.NotNil(t, flag, "expected --search flag to exist")
	})

	t.Run("attempts auto-init when no flags provided", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create task store with no root tasks
		_, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--config", filepath.Join(tmpDir, "ralph.yaml")})

		// Change to temp dir to avoid polluting working directory
		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		// Should fail with auto-init error (no root tasks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "No tasks")
	})

	t.Run("--parent creates .ralph directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a task store with a parent task
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("feature-1", "Test Feature")
		require.NoError(t, store.Save(parentTask))

		// Create a leaf task under the parent
		leafTask := createValidLeafTask("leaf-1", "Leaf Task", strPtr("feature-1"))
		require.NoError(t, store.Save(leafTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "feature-1"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		// Verify .ralph directory structure exists
		assert.DirExists(t, state.RalphDirPath(tmpDir))
		assert.DirExists(t, state.TasksDirPath(tmpDir))
		assert.DirExists(t, state.StateDirPath(tmpDir))
		assert.DirExists(t, state.LogsDirPath(tmpDir))
		assert.DirExists(t, state.ArchiveDirPath(tmpDir))
	})

	t.Run("--parent writes parent-task-id file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a task store with a parent task
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("feature-x", "Feature X")
		require.NoError(t, store.Save(parentTask))

		// Create a leaf task
		leafTask := createValidLeafTask("leaf-x", "Leaf X", strPtr("feature-x"))
		require.NoError(t, store.Save(leafTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "feature-x"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		// Verify parent-task-id file contains the correct ID
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		data, err := os.ReadFile(parentIDFile)
		require.NoError(t, err)
		assert.Equal(t, "feature-x", string(data))
	})

	t.Run("--parent validates task exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create empty task store
		_, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "nonexistent"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("validates graph has no cycles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a task store with cyclic dependencies
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		// Create parent task
		parentTask := createValidParentTask("parent", "Parent")
		require.NoError(t, store.Save(parentTask))

		// Create tasks with cyclic dependencies
		taskA := createValidLeafTask("task-a", "Task A", strPtr("parent"))
		taskA.DependsOn = []string{"task-b"} // A depends on B
		require.NoError(t, store.Save(taskA))

		taskB := createValidLeafTask("task-b", "Task B", strPtr("parent"))
		taskB.DependsOn = []string{"task-a"} // B depends on A - cycle!
		require.NoError(t, store.Save(taskB))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "parent"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle")
	})

	t.Run("validates graph has ready leaves", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a task store where no leaves are ready
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		// Create parent task
		parentTask := createValidParentTask("parent", "Parent")
		require.NoError(t, store.Save(parentTask))

		// Create leaf task that depends on non-existent completed task
		leafTask := createValidLeafTask("leaf", "Leaf", strPtr("parent"))
		require.NoError(t, store.Save(leafTask))

		// Create dep task that is not completed
		depTask := createValidLeafTask("dep", "Dependency", strPtr("parent"))
		require.NoError(t, store.Save(depTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "parent"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		// This should succeed - dep task IS a ready leaf (it has no deps and is open)
		require.NoError(t, err)
	})

	t.Run("validates graph has ready leaves - no leaves ready", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a task store where no leaves are ready (all completed)
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		// Create parent task
		parentTask := createValidParentTask("parent", "Parent")
		require.NoError(t, store.Save(parentTask))

		// Create leaf task that is already completed
		leafTask := createValidLeafTask("leaf", "Leaf", strPtr("parent"))
		leafTask.Status = taskstore.StatusCompleted // Make it completed so it's not ready
		require.NoError(t, store.Save(leafTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "parent"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no ready leaf")
	})

	t.Run("--search finds task by title", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a task store with tasks
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("feature-auth", "Authentication Feature")
		require.NoError(t, store.Save(parentTask))

		leafTask := createValidLeafTask("auth-leaf", "Auth Leaf", strPtr("feature-auth"))
		require.NoError(t, store.Save(leafTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--search", "Authentication"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		// Verify parent-task-id file contains the found ID
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		data, err := os.ReadFile(parentIDFile)
		require.NoError(t, err)
		assert.Equal(t, "feature-auth", string(data))
	})

	t.Run("--search is case insensitive", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("my-feature", "My AWESOME Feature")
		require.NoError(t, store.Save(parentTask))

		leafTask := createValidLeafTask("leaf-1", "Leaf", strPtr("my-feature"))
		require.NoError(t, store.Save(leafTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--search", "awesome"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		data, err := os.ReadFile(parentIDFile)
		require.NoError(t, err)
		assert.Equal(t, "my-feature", string(data))
	})

	t.Run("--search returns error if not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create empty task store
		_, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--search", "nonexistent"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no task found")
	})

	t.Run("--search returns error if multiple matches", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		task1 := createValidParentTask("feature-1", "Feature One")
		require.NoError(t, store.Save(task1))

		task2 := createValidParentTask("feature-2", "Feature Two")
		require.NoError(t, store.Save(task2))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--search", "Feature"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple tasks")
	})

	t.Run("cannot specify both --parent and --search", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "x", "--search", "y"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both")
	})

	t.Run("prints success message", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("test-feature", "Test Feature")
		require.NoError(t, store.Save(parentTask))

		leafTask := createValidLeafTask("leaf", "Leaf", strPtr("test-feature"))
		require.NoError(t, store.Save(leafTask))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "test-feature"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("archives progress when parent task changes", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		// Create first parent task
		parentTask1 := createValidParentTask("feature-1", "Feature One")
		require.NoError(t, store.Save(parentTask1))
		leafTask1 := createValidLeafTask("leaf-1", "Leaf 1", strPtr("feature-1"))
		require.NoError(t, store.Save(leafTask1))

		// Initialize with first parent
		cmd1 := NewRootCmd()
		cmd1.SetArgs([]string{"init", "--parent", "feature-1"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd1.Execute()
		require.NoError(t, err)

		// Verify progress file was created
		progressPath := filepath.Join(tmpDir, ".ralph", "progress.md")
		require.FileExists(t, progressPath)

		// Add some content to progress file
		originalContent := "# Some progress content\n"
		err = os.WriteFile(progressPath, []byte(originalContent), 0644)
		require.NoError(t, err)

		// Create second parent task
		parentTask2 := createValidParentTask("feature-2", "Feature Two")
		require.NoError(t, store.Save(parentTask2))
		leafTask2 := createValidLeafTask("leaf-2", "Leaf 2", strPtr("feature-2"))
		require.NoError(t, store.Save(leafTask2))

		// Initialize with second parent (should archive old progress)
		cmd2 := NewRootCmd()
		cmd2.SetArgs([]string{"init", "--parent", "feature-2"})

		err = cmd2.Execute()
		require.NoError(t, err)

		// Verify old progress was archived
		archiveDir := filepath.Join(tmpDir, ".ralph", "archive")
		entries, err := os.ReadDir(archiveDir)
		require.NoError(t, err)
		require.Len(t, entries, 1, "expected one archived progress file")
		assert.Contains(t, entries[0].Name(), "progress-")
		assert.Contains(t, entries[0].Name(), ".md")

		// Verify archived content
		archivedPath := filepath.Join(archiveDir, entries[0].Name())
		archivedContent, err := os.ReadFile(archivedPath)
		require.NoError(t, err)
		assert.Equal(t, originalContent, string(archivedContent))

		// Verify new progress file was created
		require.FileExists(t, progressPath)
		newContent, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		assert.Contains(t, string(newContent), "Feature Two")
		assert.Contains(t, string(newContent), "feature-2")
	})

	t.Run("does not archive progress when parent task stays the same", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("feature-x", "Feature X")
		require.NoError(t, store.Save(parentTask))
		leafTask := createValidLeafTask("leaf-x", "Leaf X", strPtr("feature-x"))
		require.NoError(t, store.Save(leafTask))

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		// Initialize first time
		cmd1 := NewRootCmd()
		cmd1.SetArgs([]string{"init", "--parent", "feature-x"})
		err = cmd1.Execute()
		require.NoError(t, err)

		progressPath := filepath.Join(tmpDir, ".ralph", "progress.md")
		require.FileExists(t, progressPath)

		// Initialize again with same parent
		cmd2 := NewRootCmd()
		cmd2.SetArgs([]string{"init", "--parent", "feature-x"})
		err = cmd2.Execute()
		require.NoError(t, err)

		// Verify no archive was created
		archiveDir := filepath.Join(tmpDir, ".ralph", "archive")
		entries, err := os.ReadDir(archiveDir)
		require.NoError(t, err)
		assert.Len(t, entries, 0, "expected no archived progress files")
	})

	t.Run("creates progress file on first init", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := createValidParentTask("initial-feature", "Initial Feature")
		require.NoError(t, store.Save(parentTask))
		leafTask := createValidLeafTask("initial-leaf", "Initial Leaf", strPtr("initial-feature"))
		require.NoError(t, store.Save(leafTask))

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--parent", "initial-feature"})
		err = cmd.Execute()
		require.NoError(t, err)

		// Verify progress file was created
		progressPath := filepath.Join(tmpDir, ".ralph", "progress.md")
		require.FileExists(t, progressPath)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "Initial Feature")
		assert.Contains(t, string(content), "initial-feature")
		assert.Contains(t, string(content), "## Codebase Patterns")
		assert.Contains(t, string(content), "## Iteration Log")
	})
}

// strPtr is a helper to create string pointers
func strPtr(s string) *string {
	return &s
}

// Auto-init tests

func TestInitCmd_AutoInit_SingleRootTask(t *testing.T) {
	tmpDir, _ := setupTestDirWithTasks(t, 1)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify parent-task-id was written
	parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
	data, err := os.ReadFile(parentIDFile)
	require.NoError(t, err)
	assert.Equal(t, "root-1", string(data))

	// Verify state was updated
	storedID, err := state.GetStoredParentTaskID(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "root-1", storedID)

	// Verify output message
	output := outBuf.String()
	assert.Contains(t, output, "Initializing:")
	assert.Contains(t, output, "Root Task 1")
	assert.Contains(t, output, "root-1")
}

func TestInitCmd_AutoInit_ZeroRootTasks(t *testing.T) {
	_, _ = setupTestDirWithTasks(t, 0)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No tasks")
}

func TestInitCmd_AutoInit_MultipleRoots_Interactive(t *testing.T) {
	tmpDir, _ := setupTestDirWithTasks(t, 3)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})

	// Mock stdin with selection "2\n"
	inputBuf := bytes.NewBufferString("2\n")
	cmd.SetIn(inputBuf)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify correct task was selected
	parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
	data, err := os.ReadFile(parentIDFile)
	require.NoError(t, err)
	assert.Equal(t, "root-2", string(data))

	// Verify menu was displayed
	output := outBuf.String()
	assert.Contains(t, output, "Select a root task")
	assert.Contains(t, output, "1) Root Task 1")
	assert.Contains(t, output, "2) Root Task 2")
	assert.Contains(t, output, "3) Root Task 3")
}

func TestInitCmd_AutoInit_MultipleRoots_NonTTY(t *testing.T) {
	_, _ = setupTestDirWithTasks(t, 3)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})

	// Use default stdin (non-TTY in test env)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple root tasks found")
	assert.Contains(t, err.Error(), "Root Task 1")
	assert.Contains(t, err.Error(), "Root Task 2")
	assert.Contains(t, err.Error(), "Root Task 3")
	assert.Contains(t, err.Error(), "--parent")
}

func TestInitCmd_AutoInit_UserCancelsSelection(t *testing.T) {
	_, _ = setupTestDirWithTasks(t, 3)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})

	// Mock stdin with "q\n" (quit)
	inputBuf := bytes.NewBufferString("q\n")
	cmd.SetIn(inputBuf)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestInitCmd_AutoInit_InvalidSelection(t *testing.T) {
	_, _ = setupTestDirWithTasks(t, 3)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"init"})

	// Mock stdin with invalid selection
	inputBuf := bytes.NewBufferString("99\n")
	cmd.SetIn(inputBuf)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestInitCmd_ShowsDeprecationWarning(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	cmd := newInitCmd()
	cmd.SetArgs([]string{})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	_ = cmd.Execute() // Ignore error, we just want to check stderr

	// Check that deprecation warning was written to stderr
	assert.Contains(t, errBuf.String(), "Deprecated:")
	assert.Contains(t, errBuf.String(), "ralph (auto-initializes) or ralph --parent <id>")
}

func TestInitCmd_DeprecationWarningDoesNotPreventExecution(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create task store with a parent task
	store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
	require.NoError(t, err)

	parentTask := createValidParentTask("test-feature", "Test Feature")
	require.NoError(t, store.Save(parentTask))

	// Create a leaf task under the parent
	leafTask := createValidLeafTask("leaf-1", "Leaf Task", strPtr("test-feature"))
	require.NoError(t, store.Save(leafTask))

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--parent", "test-feature"})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	require.NoError(t, err)

	// Warning should be on stderr
	assert.Contains(t, errBuf.String(), "Deprecated:")

	// Success output should be on stdout
	assert.Contains(t, outBuf.String(), "Initialized ralph for parent task")
}
