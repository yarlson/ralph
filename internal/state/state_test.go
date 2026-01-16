package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureRalphDir(t *testing.T) {
	t.Run("creates all directories if missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := EnsureRalphDir(tmpDir)
		require.NoError(t, err)

		// Verify all expected directories exist
		expectedDirs := []string{
			".ralph",
			".ralph/tasks",
			".ralph/state",
			".ralph/logs",
			".ralph/logs/claude",
			".ralph/archive",
		}

		for _, dir := range expectedDirs {
			fullPath := filepath.Join(tmpDir, dir)
			info, err := os.Stat(fullPath)
			assert.NoError(t, err, "directory %s should exist", dir)
			assert.True(t, info.IsDir(), "%s should be a directory", dir)
		}
	})

	t.Run("is idempotent - calling twice succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Call twice
		err := EnsureRalphDir(tmpDir)
		require.NoError(t, err)

		err = EnsureRalphDir(tmpDir)
		require.NoError(t, err)

		// Verify directories still exist
		ralphDir := filepath.Join(tmpDir, ".ralph")
		info, err := os.Stat(ralphDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("directories have correct permissions", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := EnsureRalphDir(tmpDir)
		require.NoError(t, err)

		// Check that directories are readable/writable by owner
		dirs := []string{
			".ralph",
			".ralph/tasks",
			".ralph/state",
			".ralph/logs",
			".ralph/logs/claude",
			".ralph/archive",
		}

		for _, dir := range dirs {
			fullPath := filepath.Join(tmpDir, dir)
			info, err := os.Stat(fullPath)
			require.NoError(t, err)

			// Check that directory has at least rwx for owner (0700)
			perm := info.Mode().Perm()
			assert.True(t, perm&0700 == 0700, "directory %s should have rwx for owner, got %o", dir, perm)
		}
	})

	t.Run("returns error for invalid root path", func(t *testing.T) {
		// Try to create in a path that doesn't exist
		invalidPath := "/nonexistent/path/that/should/not/exist"

		err := EnsureRalphDir(invalidPath)
		assert.Error(t, err)
	})

	t.Run("works when some directories already exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pre-create some directories
		err := os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755)
		require.NoError(t, err)

		// Now call EnsureRalphDir
		err = EnsureRalphDir(tmpDir)
		require.NoError(t, err)

		// Verify all directories exist
		expectedDirs := []string{
			".ralph",
			".ralph/tasks",
			".ralph/state",
			".ralph/logs",
			".ralph/logs/claude",
			".ralph/archive",
		}

		for _, dir := range expectedDirs {
			fullPath := filepath.Join(tmpDir, dir)
			info, err := os.Stat(fullPath)
			assert.NoError(t, err, "directory %s should exist", dir)
			assert.True(t, info.IsDir(), "%s should be a directory", dir)
		}
	})
}

func TestRalphDirPath(t *testing.T) {
	t.Run("returns correct path for subdirectory", func(t *testing.T) {
		root := "/some/project"

		assert.Equal(t, "/some/project/.ralph", RalphDirPath(root))
		assert.Equal(t, "/some/project/.ralph/tasks", TasksDirPath(root))
		assert.Equal(t, "/some/project/.ralph/state", StateDirPath(root))
		assert.Equal(t, "/some/project/.ralph/logs", LogsDirPath(root))
		assert.Equal(t, "/some/project/.ralph/logs/claude", ClaudeLogsDirPath(root))
		assert.Equal(t, "/some/project/.ralph/archive", ArchiveDirPath(root))
	})
}

func TestPausedFilePath(t *testing.T) {
	root := "/some/project"
	expected := "/some/project/.ralph/state/paused"
	assert.Equal(t, expected, PausedFilePath(root))
}

func TestIsPaused(t *testing.T) {
	t.Run("returns error when state dir does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		paused, err := IsPaused(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ".ralph/state")
		assert.False(t, paused)
	})

	t.Run("returns false when not paused", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		paused, err := IsPaused(tmpDir)
		require.NoError(t, err)
		assert.False(t, paused)
	})

	t.Run("returns true when paused", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		// Create paused file
		pausedPath := PausedFilePath(tmpDir)
		require.NoError(t, os.WriteFile(pausedPath, []byte{}, 0644))

		paused, err := IsPaused(tmpDir)
		require.NoError(t, err)
		assert.True(t, paused)
	})
}

func TestSetPaused(t *testing.T) {
	t.Run("returns error when state dir does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := SetPaused(tmpDir, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ".ralph/state")
	})

	t.Run("creates paused file when setting paused to true", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		err := SetPaused(tmpDir, true)
		require.NoError(t, err)

		// Verify file exists
		pausedPath := PausedFilePath(tmpDir)
		_, err = os.Stat(pausedPath)
		assert.NoError(t, err)
	})

	t.Run("removes paused file when setting paused to false", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		// Create paused file first
		pausedPath := PausedFilePath(tmpDir)
		require.NoError(t, os.WriteFile(pausedPath, []byte{}, 0644))

		err := SetPaused(tmpDir, false)
		require.NoError(t, err)

		// Verify file was removed
		_, err = os.Stat(pausedPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("succeeds when removing non-existent paused file", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		err := SetPaused(tmpDir, false)
		require.NoError(t, err)
	})

	t.Run("is idempotent - setting paused twice succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		require.NoError(t, SetPaused(tmpDir, true))
		require.NoError(t, SetPaused(tmpDir, true))

		paused, err := IsPaused(tmpDir)
		require.NoError(t, err)
		assert.True(t, paused)
	})
}

func TestGetStoredParentTaskID(t *testing.T) {
	t.Run("returns empty string when file doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		taskID, err := GetStoredParentTaskID(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, "", taskID)
	})

	t.Run("reads stored parent task ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		// Write a task ID
		require.NoError(t, SetStoredParentTaskID(tmpDir, "task-123"))

		// Read it back
		taskID, err := GetStoredParentTaskID(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, "task-123", taskID)
	})
}

func TestSetStoredParentTaskID(t *testing.T) {
	t.Run("returns error when state dir does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := SetStoredParentTaskID(tmpDir, "task-123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ".ralph/state")
	})

	t.Run("writes parent task ID to state file", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		err := SetStoredParentTaskID(tmpDir, "my-task")
		require.NoError(t, err)

		// Verify file was written
		path := ParentTaskIDFilePath(tmpDir)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "my-task", string(data))
	})

	t.Run("overwrites existing task ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, EnsureRalphDir(tmpDir))

		// Write first ID
		require.NoError(t, SetStoredParentTaskID(tmpDir, "task-1"))

		// Overwrite with second ID
		require.NoError(t, SetStoredParentTaskID(tmpDir, "task-2"))

		// Verify second ID is stored
		taskID, err := GetStoredParentTaskID(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, "task-2", taskID)
	})
}
