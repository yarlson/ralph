// Package state manages the .ralph directory structure and state files.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Directory names for the .ralph structure.
const (
	RalphDir      = ".ralph"
	TasksDir      = "tasks"
	StateDir      = "state"
	LogsDir       = "logs"
	ClaudeLogsDir = "claude"
	ArchiveDir    = "archive"
	PausedFile    = "paused"
)

// RalphDirPath returns the path to the .ralph directory.
func RalphDirPath(root string) string {
	return filepath.Join(root, RalphDir)
}

// TasksDirPath returns the path to the tasks directory.
func TasksDirPath(root string) string {
	return filepath.Join(root, RalphDir, TasksDir)
}

// StateDirPath returns the path to the state directory.
func StateDirPath(root string) string {
	return filepath.Join(root, RalphDir, StateDir)
}

// LogsDirPath returns the path to the logs directory.
func LogsDirPath(root string) string {
	return filepath.Join(root, RalphDir, LogsDir)
}

// ClaudeLogsDirPath returns the path to the Claude logs directory.
func ClaudeLogsDirPath(root string) string {
	return filepath.Join(root, RalphDir, LogsDir, ClaudeLogsDir)
}

// ArchiveDirPath returns the path to the archive directory.
func ArchiveDirPath(root string) string {
	return filepath.Join(root, RalphDir, ArchiveDir)
}

// EnsureRalphDir creates the .ralph directory structure if it doesn't exist.
// It creates the following directories:
//   - .ralph/
//   - .ralph/tasks/
//   - .ralph/state/
//   - .ralph/logs/
//   - .ralph/logs/claude/
//   - .ralph/archive/
//
// The function is idempotent - calling it multiple times is safe.
// All directories are created with 0755 permissions (rwxr-xr-x).
func EnsureRalphDir(root string) error {
	// Verify root exists
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return fmt.Errorf("root directory does not exist: %s", root)
	}

	// Directories to create in order (parent dirs first)
	dirs := []string{
		RalphDirPath(root),
		TasksDirPath(root),
		StateDirPath(root),
		LogsDirPath(root),
		ClaudeLogsDirPath(root),
		ArchiveDirPath(root),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// PausedFilePath returns the path to the paused state file.
func PausedFilePath(root string) string {
	return filepath.Join(root, RalphDir, StateDir, PausedFile)
}

// IsPaused checks if the loop is currently paused.
func IsPaused(root string) (bool, error) {
	stateDir := StateDirPath(root)
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return false, fmt.Errorf(".ralph/state directory does not exist")
	}

	pausedPath := PausedFilePath(root)
	_, err := os.Stat(pausedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check paused state: %w", err)
	}
	return true, nil
}

// SetPaused sets the paused state.
func SetPaused(root string, paused bool) error {
	stateDir := StateDirPath(root)
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return fmt.Errorf(".ralph/state directory does not exist")
	}

	pausedPath := PausedFilePath(root)

	if paused {
		// Create paused file
		file, err := os.Create(pausedPath)
		if err != nil {
			return fmt.Errorf("failed to create paused file: %w", err)
		}
		return file.Close()
	}

	// Remove paused file
	err := os.Remove(pausedPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove paused file: %w", err)
	}
	return nil
}
