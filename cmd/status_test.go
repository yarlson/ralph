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

func TestStatusCommand(t *testing.T) {
	t.Run("command exists and has correct structure", func(t *testing.T) {
		cmd := newStatusCmd()
		assert.Equal(t, "status", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
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

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

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
		assert.Contains(t, output, "Parent: parent")
		assert.Contains(t, output, "Total: 0")
	})

	t.Run("displays task counts correctly", func(t *testing.T) {
		tmpDir := t.TempDir()

		store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
		require.NoError(t, err)

		parentID := "feature"
		parentTask := &taskstore.Task{
			ID:        parentID,
			Title:     "Feature",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(parentTask))

		completedTask := &taskstore.Task{
			ID:        "completed",
			Title:     "Completed Task",
			Status:    taskstore.StatusCompleted,
			ParentID:  &parentID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(completedTask))

		readyTask := &taskstore.Task{
			ID:        "ready",
			Title:     "Ready Task",
			Status:    taskstore.StatusOpen,
			ParentID:  &parentID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(readyTask))

		failedTask := &taskstore.Task{
			ID:        "failed",
			Title:     "Failed Task",
			Status:    taskstore.StatusFailed,
			ParentID:  &parentID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(failedTask))

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755))

		parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
		require.NoError(t, os.WriteFile(parentIDFile, []byte(parentID), 0644))

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
}
