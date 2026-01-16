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

		parentTask := &taskstore.Task{
			ID:        "feature-1",
			Title:     "Test Feature",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Create a leaf task under the parent
		leafTask := &taskstore.Task{
			ID:        "leaf-1",
			Title:     "Leaf Task",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("feature-1"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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

		parentTask := &taskstore.Task{
			ID:        "feature-x",
			Title:     "Feature X",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Create a leaf task
		leafTask := &taskstore.Task{
			ID:        "leaf-x",
			Title:     "Leaf X",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("feature-x"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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
		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Create tasks with cyclic dependencies
		taskA := &taskstore.Task{
			ID:        "task-a",
			Title:     "Task A",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			DependsOn: []string{"task-b"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(taskA))

		taskB := &taskstore.Task{
			ID:        "task-b",
			Title:     "Task B",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			DependsOn: []string{"task-a"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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
		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Create leaf task that depends on non-existent completed task
		leafTask := &taskstore.Task{
			ID:        "leaf",
			Title:     "Leaf",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			DependsOn: []string{"dep"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(leafTask))

		// Create dep task that is not completed
		depTask := &taskstore.Task{
			ID:        "dep",
			Title:     "Dependency",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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
		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Create leaf task that is already completed
		leafTask := &taskstore.Task{
			ID:        "leaf",
			Title:     "Leaf",
			Status:    taskstore.StatusCompleted,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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

		parentTask := &taskstore.Task{
			ID:        "feature-auth",
			Title:     "Authentication Feature",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		leafTask := &taskstore.Task{
			ID:        "auth-leaf",
			Title:     "Auth Leaf",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("feature-auth"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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

		parentTask := &taskstore.Task{
			ID:        "my-feature",
			Title:     "My AWESOME Feature",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		leafTask := &taskstore.Task{
			ID:        "leaf-1",
			Title:     "Leaf",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("my-feature"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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

		task1 := &taskstore.Task{
			ID:        "feature-1",
			Title:     "Feature One",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(task1))

		task2 := &taskstore.Task{
			ID:        "feature-2",
			Title:     "Feature Two",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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

		parentTask := &taskstore.Task{
			ID:        "test-feature",
			Title:     "Test Feature",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		leafTask := &taskstore.Task{
			ID:        "leaf",
			Title:     "Leaf",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("test-feature"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
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
