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
