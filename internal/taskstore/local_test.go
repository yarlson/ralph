package taskstore

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTask(id string) *Task {
	now := time.Now().Truncate(time.Second)
	return &Task{
		ID:        id,
		Title:     "Test Task " + id,
		Status:    StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewLocalStore(t *testing.T) {
	t.Run("creates directory if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")

		_, err := NewLocalStore(tasksDir)
		require.NoError(t, err)

		info, err := os.Stat(tasksDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("works with existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		tasksDir := filepath.Join(tmpDir, "tasks")
		require.NoError(t, os.MkdirAll(tasksDir, 0755))

		_, err := NewLocalStore(tasksDir)
		require.NoError(t, err)
	})

	t.Run("fails with invalid path", func(t *testing.T) {
		// Try to create directory at path that's a file
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		_, err := NewLocalStore(filePath)
		assert.Error(t, err)
	})
}

func TestLocalStore_Save(t *testing.T) {
	t.Run("saves new task", func(t *testing.T) {
		store := newTestStore(t)
		task := newTestTask("task-1")

		err := store.Save(task)
		require.NoError(t, err)

		// Verify file exists
		taskFile := filepath.Join(store.dir, "task-1.json")
		_, err = os.Stat(taskFile)
		assert.NoError(t, err)
	})

	t.Run("updates existing task", func(t *testing.T) {
		store := newTestStore(t)
		task := newTestTask("task-1")
		require.NoError(t, store.Save(task))

		// Update task
		task.Title = "Updated Title"
		task.UpdatedAt = time.Now().Truncate(time.Second)
		err := store.Save(task)
		require.NoError(t, err)

		// Verify update persisted
		loaded, err := store.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", loaded.Title)
	})

	t.Run("fails on validation error", func(t *testing.T) {
		store := newTestStore(t)
		task := &Task{
			ID: "", // Empty ID should fail validation
		}

		err := store.Save(task)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("updates UpdatedAt timestamp", func(t *testing.T) {
		store := newTestStore(t)
		oldTime := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
		task := &Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    StatusOpen,
			CreatedAt: oldTime,
			UpdatedAt: oldTime,
		}
		require.NoError(t, store.Save(task))

		// Save again - UpdatedAt should be updated
		task.Title = "Changed"
		err := store.Save(task)
		require.NoError(t, err)

		loaded, err := store.Get("task-1")
		require.NoError(t, err)
		assert.True(t, loaded.UpdatedAt.After(oldTime))
	})
}

func TestLocalStore_Get(t *testing.T) {
	t.Run("retrieves existing task", func(t *testing.T) {
		store := newTestStore(t)
		task := newTestTask("task-1")
		task.Description = "A test task"
		task.Labels = map[string]string{"area": "core"}
		require.NoError(t, store.Save(task))

		loaded, err := store.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, task.ID, loaded.ID)
		assert.Equal(t, task.Title, loaded.Title)
		assert.Equal(t, task.Description, loaded.Description)
		assert.Equal(t, task.Status, loaded.Status)
		assert.Equal(t, task.Labels, loaded.Labels)
	})

	t.Run("returns NotFoundError for missing task", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.Get("nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestLocalStore_List(t *testing.T) {
	t.Run("returns empty list for empty store", func(t *testing.T) {
		store := newTestStore(t)

		tasks, err := store.List()
		require.NoError(t, err)
		assert.Empty(t, tasks)
	})

	t.Run("returns all tasks", func(t *testing.T) {
		store := newTestStore(t)
		require.NoError(t, store.Save(newTestTask("task-1")))
		require.NoError(t, store.Save(newTestTask("task-2")))
		require.NoError(t, store.Save(newTestTask("task-3")))

		tasks, err := store.List()
		require.NoError(t, err)
		assert.Len(t, tasks, 3)

		ids := make(map[string]bool)
		for _, task := range tasks {
			ids[task.ID] = true
		}
		assert.True(t, ids["task-1"])
		assert.True(t, ids["task-2"])
		assert.True(t, ids["task-3"])
	})

	t.Run("ignores non-json files", func(t *testing.T) {
		store := newTestStore(t)
		require.NoError(t, store.Save(newTestTask("task-1")))

		// Add a non-json file
		otherFile := filepath.Join(store.dir, "readme.txt")
		require.NoError(t, os.WriteFile(otherFile, []byte("not a task"), 0644))

		tasks, err := store.List()
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
	})
}

func TestLocalStore_ListByParent(t *testing.T) {
	t.Run("returns tasks with matching parent", func(t *testing.T) {
		store := newTestStore(t)

		parentID := "parent-1"
		child1 := newTestTask("child-1")
		child1.ParentID = &parentID
		child2 := newTestTask("child-2")
		child2.ParentID = &parentID
		orphan := newTestTask("orphan")

		require.NoError(t, store.Save(child1))
		require.NoError(t, store.Save(child2))
		require.NoError(t, store.Save(orphan))

		tasks, err := store.ListByParent("parent-1")
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
	})

	t.Run("returns root tasks for empty parent", func(t *testing.T) {
		store := newTestStore(t)

		parentID := "parent-1"
		child := newTestTask("child-1")
		child.ParentID = &parentID
		root := newTestTask("root")

		require.NoError(t, store.Save(child))
		require.NoError(t, store.Save(root))

		tasks, err := store.ListByParent("")
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, "root", tasks[0].ID)
	})
}

func TestLocalStore_UpdateStatus(t *testing.T) {
	t.Run("updates status of existing task", func(t *testing.T) {
		store := newTestStore(t)
		task := newTestTask("task-1")
		require.NoError(t, store.Save(task))

		err := store.UpdateStatus("task-1", StatusCompleted)
		require.NoError(t, err)

		loaded, err := store.Get("task-1")
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, loaded.Status)
	})

	t.Run("returns NotFoundError for missing task", func(t *testing.T) {
		store := newTestStore(t)

		err := store.UpdateStatus("nonexistent", StatusCompleted)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("updates UpdatedAt timestamp", func(t *testing.T) {
		store := newTestStore(t)
		oldTime := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
		task := &Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    StatusOpen,
			CreatedAt: oldTime,
			UpdatedAt: oldTime,
		}
		require.NoError(t, store.Save(task))

		err := store.UpdateStatus("task-1", StatusCompleted)
		require.NoError(t, err)

		loaded, err := store.Get("task-1")
		require.NoError(t, err)
		assert.True(t, loaded.UpdatedAt.After(oldTime))
	})
}

func TestLocalStore_Delete(t *testing.T) {
	t.Run("deletes existing task", func(t *testing.T) {
		store := newTestStore(t)
		require.NoError(t, store.Save(newTestTask("task-1")))

		err := store.Delete("task-1")
		require.NoError(t, err)

		_, err = store.Get("task-1")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("returns NotFoundError for missing task", func(t *testing.T) {
		store := newTestStore(t)

		err := store.Delete("nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestLocalStore_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent saves", func(t *testing.T) {
		store := newTestStore(t)
		var wg sync.WaitGroup
		numTasks := 20

		for i := 0; i < numTasks; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				task := newTestTask("task-" + string(rune('a'+id)))
				err := store.Save(task)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		tasks, err := store.List()
		require.NoError(t, err)
		assert.Len(t, tasks, numTasks)
	})

	t.Run("handles concurrent reads and writes", func(t *testing.T) {
		store := newTestStore(t)
		require.NoError(t, store.Save(newTestTask("task-1")))

		var wg sync.WaitGroup
		numOperations := 50

		for i := 0; i < numOperations; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, err := store.Get("task-1")
				assert.NoError(t, err)
			}()
			go func(n int) {
				defer wg.Done()
				task := newTestTask("task-1")
				task.Title = "Updated " + string(rune('a'+n))
				err := store.Save(task)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
	})
}

func TestLocalStore_AtomicWrite(t *testing.T) {
	t.Run("does not leave partial files on error", func(t *testing.T) {
		store := newTestStore(t)
		task := newTestTask("task-1")
		require.NoError(t, store.Save(task))

		// Verify temp files are cleaned up
		files, err := os.ReadDir(store.dir)
		require.NoError(t, err)

		for _, f := range files {
			assert.False(t, filepath.Ext(f.Name()) == ".tmp", "temp file should be cleaned up: %s", f.Name())
		}
	})
}

// newTestStore creates a LocalStore with a temporary directory for testing.
func newTestStore(t *testing.T) *LocalStore {
	t.Helper()
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	store, err := NewLocalStore(tasksDir)
	require.NoError(t, err)
	return store
}
