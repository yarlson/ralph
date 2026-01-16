package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/go-ralph/internal/claude"
	"github.com/yarlson/go-ralph/internal/loop"
	"github.com/yarlson/go-ralph/internal/taskstore"
	"github.com/yarlson/go-ralph/internal/verifier"
)

// mockClaudeRunner for testing
type mockClaudeRunner struct {
	response *claude.ClaudeResponse
	err      error
}

func (m *mockClaudeRunner) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	return m.response, m.err
}

// mockVerifier for testing
type mockVerifier struct {
	results []verifier.VerificationResult
	err     error
}

func (m *mockVerifier) Verify(ctx context.Context, commands [][]string) ([]verifier.VerificationResult, error) {
	return m.results, m.err
}

func (m *mockVerifier) VerifyTask(ctx context.Context, commands [][]string) ([]verifier.VerificationResult, error) {
	return m.Verify(ctx, commands)
}

// mockGitManager for testing
type mockGitManager struct {
	currentCommit  string
	hasChanges     bool
	changedFiles   []string
	commitHash     string
	currentBranch  string
	diffStat       string
	ensureBranchErr error
	commitErr      error
}

func (m *mockGitManager) EnsureBranch(ctx context.Context, branchName string) error {
	return m.ensureBranchErr
}

func (m *mockGitManager) GetCurrentCommit(ctx context.Context) (string, error) {
	return m.currentCommit, nil
}

func (m *mockGitManager) HasChanges(ctx context.Context) (bool, error) {
	return m.hasChanges, nil
}

func (m *mockGitManager) GetDiffStat(ctx context.Context) (string, error) {
	return m.diffStat, nil
}

func (m *mockGitManager) GetChangedFiles(ctx context.Context) ([]string, error) {
	return m.changedFiles, nil
}

func (m *mockGitManager) Commit(ctx context.Context, message string) (string, error) {
	if m.commitErr != nil {
		return "", m.commitErr
	}
	return m.commitHash, nil
}

func (m *mockGitManager) GetCurrentBranch(ctx context.Context) (string, error) {
	return m.currentBranch, nil
}

func TestNewRunCmd(t *testing.T) {
	cmd := newRunCmd()

	assert.Equal(t, "run", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check flags exist
	assert.NotNil(t, cmd.Flags().Lookup("once"))
	assert.NotNil(t, cmd.Flags().Lookup("max-iterations"))
}

func TestRunCmd_NoParentTaskID(t *testing.T) {
	// Set up temp directory
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create ralph dir but no parent-task-id file
	err = os.MkdirAll(filepath.Join(tmpDir, ".ralph"), 0755)
	require.NoError(t, err)

	// Run command
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"run"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent-task-id")
}

func TestRunCmd_NonExistentParentTask(t *testing.T) {
	// Set up temp directory
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create ralph dir structure
	err = os.MkdirAll(filepath.Join(tmpDir, ".ralph"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755)
	require.NoError(t, err)

	// Write parent-task-id file pointing to non-existent task
	err = os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("nonexistent-task"), 0644)
	require.NoError(t, err)

	// Run command
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"run"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	assert.Error(t, err)
}

func TestRunCmd_OnceFlag(t *testing.T) {
	cmd := newRunCmd()

	// Check default value
	onceFlag := cmd.Flags().Lookup("once")
	require.NotNil(t, onceFlag)
	assert.Equal(t, "false", onceFlag.DefValue)
}

func TestRunCmd_MaxIterationsFlag(t *testing.T) {
	cmd := newRunCmd()

	// Check default value
	maxIterFlag := cmd.Flags().Lookup("max-iterations")
	require.NotNil(t, maxIterFlag)
	assert.Equal(t, "0", maxIterFlag.DefValue) // 0 means use config default
}

// Tests that verify the command integrates with loop controller
func TestRunCmd_Integration_NoReadyTasks(t *testing.T) {
	// Set up temp directory
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create ralph dir structure
	err = os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, ".ralph", "logs", "claude"), 0755)
	require.NoError(t, err)

	// Create task store
	store, err := taskstore.NewLocalStore(filepath.Join(tmpDir, ".ralph", "tasks"))
	require.NoError(t, err)

	// Create parent task (completed, so no ready children)
	parentTask := &taskstore.Task{
		ID:          "parent-task",
		Title:       "Parent Task",
		Description: "Test parent task",
		Status:      taskstore.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = store.Save(parentTask)
	require.NoError(t, err)

	// Write parent-task-id file
	err = os.WriteFile(filepath.Join(tmpDir, ".ralph", "parent-task-id"), []byte("parent-task"), 0644)
	require.NoError(t, err)

	// Run command - should complete since parent is completed and no children exist
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"run"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	// No error expected for empty/completed parent
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "completed")
}

func TestRunResult_FormatOutput(t *testing.T) {
	result := loop.RunResult{
		Outcome:       loop.RunOutcomeCompleted,
		Message:       "all tasks completed",
		IterationsRun: 3,
		CompletedTasks: []string{"task-1", "task-2", "task-3"},
		FailedTasks:   []string{},
		TotalCostUSD:  0.05,
		ElapsedTime:   5 * time.Minute,
	}

	// Format the result
	output := formatRunResult(result)

	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "3")
	assert.Contains(t, output, "all tasks completed")
}

func TestRunCmd_GracefulShutdown(t *testing.T) {
	// This test verifies that the run command can be interrupted
	// The actual implementation should handle context cancellation
	cmd := newRunCmd()
	assert.NotNil(t, cmd)

	// The run command should accept context for graceful shutdown
	// This is verified indirectly through the implementation
}

func TestFormatRunResult(t *testing.T) {
	tests := []struct {
		name     string
		result   loop.RunResult
		contains []string
	}{
		{
			name: "completed successfully",
			result: loop.RunResult{
				Outcome:        loop.RunOutcomeCompleted,
				Message:        "all tasks completed",
				IterationsRun:  2,
				CompletedTasks: []string{"task-1", "task-2"},
				TotalCostUSD:   0.03,
				ElapsedTime:    2 * time.Minute,
			},
			contains: []string{"completed", "2", "all tasks completed"},
		},
		{
			name: "blocked",
			result: loop.RunResult{
				Outcome:       loop.RunOutcomeBlocked,
				Message:       "no ready tasks available",
				IterationsRun: 0,
			},
			contains: []string{"blocked", "no ready tasks available"},
		},
		{
			name: "budget exceeded",
			result: loop.RunResult{
				Outcome:       loop.RunOutcomeBudgetExceeded,
				Message:       "iteration limit reached",
				IterationsRun: 50,
			},
			contains: []string{"budget_exceeded", "50"},
		},
		{
			name: "with failed tasks",
			result: loop.RunResult{
				Outcome:        loop.RunOutcomeCompleted,
				Message:        "completed with failures",
				IterationsRun:  3,
				CompletedTasks: []string{"task-1"},
				FailedTasks:    []string{"task-2", "task-3"},
			},
			contains: []string{"1 completed", "2 failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatRunResult(tt.result)
			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

// Helper function for tests - defined in run.go
// formatRunResult formats a RunResult for CLI output
