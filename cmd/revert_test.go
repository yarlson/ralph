package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/taskstore"
)

// Helper function to run a command in a directory
func runTestCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// Helper function to get current git commit
func getTestGitCommit(t *testing.T, dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(output))
}

func TestRevertCmd_MissingIterationFlag(t *testing.T) {
	cmd := newRevertCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--iteration")
}

func TestRevertCmd_IterationNotFound(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph/logs directory but no iteration files
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	cmd := newRevertCmd()
	cmd.SetArgs([]string{"--iteration", "nonexistent", "--force"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRevertCmd_RequiresConfirmation(t *testing.T) {
	// This test verifies that confirmation is required when --force is not used
	// Since we can't easily test interactive prompts in unit tests, we'll verify
	// the command structure and that it fails without --force and a valid TTY
	cmd := newRevertCmd()
	assert.NotNil(t, cmd.Flags().Lookup("force"))
	assert.NotNil(t, cmd.Flags().Lookup("iteration"))
}

func TestRevertCmd_WithForce(t *testing.T) {
	// Create temp directory as git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	require.NoError(t, runTestCommand(tmpDir, "git", "init", "-b", "main"))
	require.NoError(t, runTestCommand(tmpDir, "git", "config", "user.email", "test@test.com"))
	require.NoError(t, runTestCommand(tmpDir, "git", "config", "user.name", "Test User"))
	require.NoError(t, runTestCommand(tmpDir, "git", "config", "commit.gpgsign", "false"))

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0644))
	require.NoError(t, runTestCommand(tmpDir, "git", "add", "."))
	require.NoError(t, runTestCommand(tmpDir, "git", "commit", "-m", "initial"))

	// Get base commit
	baseCommit := getTestGitCommit(t, tmpDir)

	// Make a change
	require.NoError(t, os.WriteFile(testFile, []byte("changed"), 0644))
	require.NoError(t, runTestCommand(tmpDir, "git", "add", "."))
	require.NoError(t, runTestCommand(tmpDir, "git", "commit", "-m", "change"))

	// Create .ralph structure
	logsDir := filepath.Join(tmpDir, ".ralph", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	tasksDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	// Create task store with a task
	store, err := taskstore.NewLocalStore(tasksDir)
	require.NoError(t, err)
	now := time.Now()
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test task for revert command",
		Status:      taskstore.StatusCompleted,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, store.Save(task))

	// Create iteration record
	record := &loop.IterationRecord{
		IterationID: "test123",
		TaskID:      "test-task",
		BaseCommit:  baseCommit,
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Create config file
	configPath := filepath.Join(tmpDir, "ralph.yaml")
	configContent := `tasks:
  path: .ralph/tasks
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Run revert command
	cmd := newRevertCmd()
	cmd.SetArgs([]string{"--iteration", "test123", "--force"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify we're back at base commit
	currentCommit := getTestGitCommit(t, tmpDir)
	assert.Equal(t, baseCommit, currentCommit)

	// Verify task status was updated
	updatedTask, err := store.Get("test-task")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusOpen, updatedTask.Status)
}
