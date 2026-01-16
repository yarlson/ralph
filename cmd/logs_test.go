package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/go-ralph/internal/loop"
	"github.com/yarlson/go-ralph/internal/state"
)

func TestLogsCmd(t *testing.T) {
	cmd := newLogsCmd()
	assert.Equal(t, "logs", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestLogsCmd_Flags(t *testing.T) {
	cmd := newLogsCmd()

	iterationFlag := cmd.Flags().Lookup("iteration")
	require.NotNil(t, iterationFlag)
	assert.Equal(t, "", iterationFlag.DefValue)
}

func TestLogsCmd_NoLogsDirectory(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command
	cmd := newLogsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No logs found")
}

func TestLogsCmd_EmptyLogsDirectory(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command
	cmd := newLogsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No iterations found")
}

func TestLogsCmd_ListIterations(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))
	logsDir := state.LogsDirPath(tmpDir)

	// Create sample iteration records
	record1 := &loop.IterationRecord{
		IterationID: "abc12345",
		TaskID:      "task-1",
		StartTime:   time.Now().Add(-2 * time.Hour),
		EndTime:     time.Now().Add(-2*time.Hour + 10*time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err := loop.SaveRecord(logsDir, record1)
	require.NoError(t, err)

	record2 := &loop.IterationRecord{
		IterationID: "def67890",
		TaskID:      "task-2",
		StartTime:   time.Now().Add(-1 * time.Hour),
		EndTime:     time.Now().Add(-1*time.Hour + 15*time.Minute),
		Outcome:     loop.OutcomeFailed,
	}
	_, err = loop.SaveRecord(logsDir, record2)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command
	cmd := newLogsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Available iterations")
	assert.Contains(t, output, "abc12345")
	assert.Contains(t, output, "task-1")
	assert.Contains(t, output, "success")
	assert.Contains(t, output, "def67890")
	assert.Contains(t, output, "task-2")
	assert.Contains(t, output, "failed")
	assert.Contains(t, output, "Use --iteration")
}

func TestLogsCmd_ShowSpecificIteration(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))
	logsDir := state.LogsDirPath(tmpDir)

	// Create sample iteration record
	record := &loop.IterationRecord{
		IterationID:  "abc12345",
		TaskID:       "task-1",
		StartTime:    time.Now().Add(-1 * time.Hour),
		EndTime:      time.Now().Add(-1*time.Hour + 10*time.Minute),
		Outcome:      loop.OutcomeSuccess,
		BaseCommit:   "commit-base",
		ResultCommit: "commit-result",
		FilesChanged: []string{"file1.go", "file2.go"},
		ClaudeInvocation: loop.ClaudeInvocationMeta{
			Model:        "claude-3-sonnet",
			SessionID:    "session-123",
			TotalCostUSD: 0.25,
			InputTokens:  1000,
			OutputTokens: 500,
		},
		VerificationOutputs: []loop.VerificationOutput{
			{
				Command: []string{"go", "test", "./..."},
				Passed:  true,
				Output:  "ok",
				Duration: 2 * time.Second,
			},
		},
		AttemptNumber: 1,
	}
	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command with --iteration flag
	cmd := newLogsCmd()
	cmd.SetArgs([]string{"--iteration", "abc12345"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Iteration: abc12345")
	assert.Contains(t, output, "Task: task-1")
	assert.Contains(t, output, "Outcome: success")
	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "Base commit: commit-base")
	assert.Contains(t, output, "Result commit: commit-result")
	assert.Contains(t, output, "Files changed:")
	assert.Contains(t, output, "file1.go")
	assert.Contains(t, output, "file2.go")
	assert.Contains(t, output, "Model: claude-3-sonnet")
	assert.Contains(t, output, "Session: session-123")
	assert.Contains(t, output, "Cost: $0.2500")
	assert.Contains(t, output, "Tokens: 1000 in / 500 out")
	assert.Contains(t, output, "Verification:")
	assert.Contains(t, output, "[PASS]")
	assert.Contains(t, output, "go test ./...")
	assert.Contains(t, output, "Attempt: 1")
}

func TestLogsCmd_ShowSpecificIteration_NotFound(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command with non-existent iteration
	cmd := newLogsCmd()
	cmd.SetArgs([]string{"--iteration", "nonexistent"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration \"nonexistent\" not found")
}

func TestLogsCmd_ShowIterationWithFailedVerification(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))
	logsDir := state.LogsDirPath(tmpDir)

	// Create iteration with failed verification
	record := &loop.IterationRecord{
		IterationID: "abc12345",
		TaskID:      "task-1",
		StartTime:   time.Now().Add(-1 * time.Hour),
		EndTime:     time.Now().Add(-1*time.Hour + 10*time.Minute),
		Outcome:     loop.OutcomeFailed,
		VerificationOutputs: []loop.VerificationOutput{
			{
				Command:  []string{"go", "test", "./..."},
				Passed:   false,
				Output:   "Error line 1\nError line 2\nError line 3",
				Duration: 2 * time.Second,
			},
		},
		Feedback: "Fix the test failures",
	}
	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command
	cmd := newLogsCmd()
	cmd.SetArgs([]string{"--iteration", "abc12345"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[FAIL]")
	assert.Contains(t, output, "Error line 1")
	assert.Contains(t, output, "Feedback:")
	assert.Contains(t, output, "Fix the test failures")
}

func TestLogsCmd_IterationsSortedByTime(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))
	logsDir := state.LogsDirPath(tmpDir)

	// Create iterations with different times (insert in reverse order)
	record3 := &loop.IterationRecord{
		IterationID: "ccc33333",
		TaskID:      "task-3",
		StartTime:   time.Now().Add(-1 * time.Hour),
		EndTime:     time.Now().Add(-1*time.Hour + 10*time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err := loop.SaveRecord(logsDir, record3)
	require.NoError(t, err)

	record2 := &loop.IterationRecord{
		IterationID: "bbb22222",
		TaskID:      "task-2",
		StartTime:   time.Now().Add(-2 * time.Hour),
		EndTime:     time.Now().Add(-2*time.Hour + 10*time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record2)
	require.NoError(t, err)

	record1 := &loop.IterationRecord{
		IterationID: "aaa11111",
		TaskID:      "task-1",
		StartTime:   time.Now().Add(-3 * time.Hour),
		EndTime:     time.Now().Add(-3*time.Hour + 10*time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err = loop.SaveRecord(logsDir, record1)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command
	cmd := newLogsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify newest first (ccc, bbb, aaa)
	cccIndex := strings.Index(output, "ccc33333")
	bbbIndex := strings.Index(output, "bbb22222")
	aaaIndex := strings.Index(output, "aaa11111")

	assert.True(t, cccIndex < bbbIndex, "ccc should appear before bbb")
	assert.True(t, bbbIndex < aaaIndex, "bbb should appear before aaa")
}

func TestFormatCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      []string
		expected string
	}{
		{
			name:     "empty command",
			cmd:      []string{},
			expected: "",
		},
		{
			name:     "single element",
			cmd:      []string{"go"},
			expected: "go",
		},
		{
			name:     "multiple elements",
			cmd:      []string{"go", "test", "./..."},
			expected: "go test ./...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCommand(tt.cmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogsCmd_MinimalIteration(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))
	logsDir := state.LogsDirPath(tmpDir)

	// Create minimal iteration record
	record := &loop.IterationRecord{
		IterationID: "min12345",
		TaskID:      "task-min",
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(5 * time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command with --iteration flag
	cmd := newLogsCmd()
	cmd.SetArgs([]string{"--iteration", "min12345"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Iteration: min12345")
	assert.Contains(t, output, "Task: task-min")
	assert.Contains(t, output, "Outcome: success")
	// Should not crash on missing optional fields
}

func TestLogsCmd_SkipsInvalidFiles(t *testing.T) {
	// Create temp directory with .ralph structure
	tmpDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(tmpDir))
	logsDir := state.LogsDirPath(tmpDir)

	// Create valid iteration record
	record := &loop.IterationRecord{
		IterationID: "valid123",
		TaskID:      "task-1",
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(10 * time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Create invalid JSON file
	invalidPath := filepath.Join(logsDir, "iteration-invalid.json")
	err = os.WriteFile(invalidPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Create non-iteration file
	otherPath := filepath.Join(logsDir, "other.txt")
	err = os.WriteFile(otherPath, []byte("other content"), 0644)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Run logs command - should not crash
	cmd := newLogsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "valid123")
	assert.NotContains(t, output, "invalid")
	assert.NotContains(t, output, "other")
}
