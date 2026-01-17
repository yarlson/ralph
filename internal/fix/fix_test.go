package fix

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/taskstore"
)

func TestService_Retry(t *testing.T) {
	t.Run("retries failed task", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		logsDir := filepath.Join(tmpDir, "logs")
		stateDir := filepath.Join(tmpDir, "state")
		require.NoError(t, os.MkdirAll(logsDir, 0755))
		require.NoError(t, os.MkdirAll(stateDir, 0755))

		store, err := taskstore.NewLocalStore(tasksDir)
		require.NoError(t, err)

		task := &taskstore.Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    taskstore.StatusFailed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(task))

		svc := NewService(store, logsDir, stateDir, tmpDir)
		err = svc.Retry("task-1", "")
		require.NoError(t, err)

		updated, _ := store.Get("task-1")
		assert.Equal(t, taskstore.StatusOpen, updated.Status)
	})

	t.Run("no-op for open task", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		logsDir := filepath.Join(tmpDir, "logs")
		stateDir := filepath.Join(tmpDir, "state")
		require.NoError(t, os.MkdirAll(logsDir, 0755))

		store, err := taskstore.NewLocalStore(tasksDir)
		require.NoError(t, err)

		task := &taskstore.Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(task))

		svc := NewService(store, logsDir, stateDir, tmpDir)
		err = svc.Retry("task-1", "")
		require.NoError(t, err)
	})

	t.Run("errors for completed task", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		logsDir := filepath.Join(tmpDir, "logs")
		stateDir := filepath.Join(tmpDir, "state")
		require.NoError(t, os.MkdirAll(logsDir, 0755))

		store, err := taskstore.NewLocalStore(tasksDir)
		require.NoError(t, err)

		task := &taskstore.Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(task))

		svc := NewService(store, logsDir, stateDir, tmpDir)
		err = svc.Retry("task-1", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot retry")
	})
}

func TestService_Skip(t *testing.T) {
	t.Run("skips open task", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		logsDir := filepath.Join(tmpDir, "logs")
		stateDir := filepath.Join(tmpDir, "state")
		require.NoError(t, os.MkdirAll(logsDir, 0755))
		require.NoError(t, os.MkdirAll(stateDir, 0755))

		store, err := taskstore.NewLocalStore(tasksDir)
		require.NoError(t, err)

		task := &taskstore.Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(task))

		svc := NewService(store, logsDir, stateDir, tmpDir)
		err = svc.Skip("task-1", "")
		require.NoError(t, err)

		updated, _ := store.Get("task-1")
		assert.Equal(t, taskstore.StatusSkipped, updated.Status)
	})

	t.Run("errors for completed task", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		logsDir := filepath.Join(tmpDir, "logs")
		stateDir := filepath.Join(tmpDir, "state")
		require.NoError(t, os.MkdirAll(logsDir, 0755))

		store, err := taskstore.NewLocalStore(tasksDir)
		require.NoError(t, err)

		task := &taskstore.Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.Save(task))

		svc := NewService(store, logsDir, stateDir, tmpDir)
		err = svc.Skip("task-1", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot skip completed")
	})
}

func TestParseEditorContent(t *testing.T) {
	t.Run("removes comment lines", func(t *testing.T) {
		input := "# Comment\nactual content\n# Another comment\nmore content"
		result := ParseEditorContent(input)
		assert.Equal(t, "actual content\nmore content", result)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		input := "\n\n  content  \n\n"
		result := ParseEditorContent(input)
		assert.Equal(t, "content", result)
	})
}
