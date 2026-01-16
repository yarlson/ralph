package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
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

	t.Run("requires either --parent or --search", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"init", "--config", filepath.Join(tmpDir, "ralph.yaml")})

		// Change to temp dir to avoid polluting working directory
		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either --parent or --search")
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
}

// strPtr is a helper to create string pointers
func strPtr(s string) *string {
	return &s
}
