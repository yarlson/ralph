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

func TestStatusCommand(t *testing.T) {
	t.Run("command exists and has correct structure", func(t *testing.T) {
		cmd := newStatusCmd()
		assert.Equal(t, "status", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
	})

	t.Run("requires parent-task-id file", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent-task-id")
	})

	t.Run("validates parent task exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .ralph directory structure
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

		// Write parent-task-id file with nonexistent task
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("nonexistent"), 0644))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("displays status with no descendant tasks", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create task store with just a parent task (no children)
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent Task",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Create .ralph structure
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

		// Write parent-task-id file
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Parent: parent")
		assert.Contains(t, output, "Total: 0")
	})

	t.Run("displays task counts correctly", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create task store with various task statuses
		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		// Parent task
		parentTask := &taskstore.Task{
			ID:        "feature",
			Title:     "Feature",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Completed task
		completedTask := &taskstore.Task{
			ID:        "completed",
			Title:     "Completed Task",
			Status:    taskstore.StatusCompleted,
			ParentID:  strPtr("feature"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(completedTask))

		// Ready leaf task (open, no deps)
		readyTask := &taskstore.Task{
			ID:        "ready",
			Title:     "Ready Task",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("feature"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(readyTask))

		// Failed task
		failedTask := &taskstore.Task{
			ID:        "failed",
			Title:     "Failed Task",
			Status:    taskstore.StatusFailed,
			ParentID:  strPtr("feature"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(failedTask))

		// Create .ralph structure
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

		// Write parent-task-id file
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("feature"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Total: 3")
		assert.Contains(t, output, "Completed: 1")
		assert.Contains(t, output, "Ready: 1")
		assert.Contains(t, output, "Failed: 1")
	})

	t.Run("displays next task", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		leafTask := &taskstore.Task{
			ID:        "next-task",
			Title:     "Next Task to Execute",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(leafTask))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "next-task")
		assert.Contains(t, output, "Next Task to Execute")
	})

	t.Run("displays last iteration info", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		leafTask := &taskstore.Task{
			ID:        "leaf",
			Title:     "Leaf",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(leafTask))

		// Create logs directory and iteration record
		logsDir := filepath.Join(tmpDir, ".ralph", "logs")
		require.NoError(t, os.MkdirAll(logsDir, 0755))

		record := loop.NewIterationRecord("leaf")
		record.IterationID = "abc12345"
		record.Outcome = loop.OutcomeSuccess
		record.EndTime = time.Now()
		_, err = loop.SaveRecord(logsDir, record)
		require.NoError(t, err)

		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Last Iteration")
		assert.Contains(t, output, "abc12345")
		assert.Contains(t, output, "success")
	})

	t.Run("handles no last iteration gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		leafTask := &taskstore.Task{
			ID:        "leaf",
			Title:     "Leaf",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(leafTask))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		// Should not error, just not show last iteration section
		output := out.String()
		assert.Contains(t, output, "Status")
	})

	t.Run("displays blocked task count", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		blockedTask := &taskstore.Task{
			ID:        "blocked",
			Title:     "Blocked Task",
			Status:    taskstore.StatusBlocked,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(blockedTask))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Blocked: 1")
	})

	t.Run("displays skipped task count", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		skippedTask := &taskstore.Task{
			ID:        "skipped",
			Title:     "Skipped Task",
			Status:    taskstore.StatusSkipped,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(skippedTask))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Skipped: 1")
	})

	t.Run("shows none when no next task", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentTask := &taskstore.Task{
			ID:        "parent",
			Title:     "Parent",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		// Only completed child, no ready tasks
		completedTask := &taskstore.Task{
			ID:        "completed",
			Title:     "Completed",
			Status:    taskstore.StatusCompleted,
			ParentID:  strPtr("parent"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(completedTask))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))
		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte("parent"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"status"})

		oldWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		err = cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Next Task: none")
	})
}
