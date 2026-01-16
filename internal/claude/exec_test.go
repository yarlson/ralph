package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildArgs_Minimal(t *testing.T) {
	req := ClaudeRequest{
		Prompt: "Hello Claude",
	}
	args := buildArgs(req)

	assert.Contains(t, args, "--output-format=stream-json")
	assert.Contains(t, args, "-p")
	// Prompt should follow -p flag
	pIndex := indexOf(args, "-p")
	require.NotEqual(t, -1, pIndex)
	require.Less(t, pIndex+1, len(args))
	assert.Equal(t, "Hello Claude", args[pIndex+1])
}

func TestBuildArgs_WithSystemPrompt(t *testing.T) {
	req := ClaudeRequest{
		Prompt:       "Hello",
		SystemPrompt: "You are a helpful assistant",
	}
	args := buildArgs(req)

	assert.Contains(t, args, "--system-prompt")
	spIndex := indexOf(args, "--system-prompt")
	require.NotEqual(t, -1, spIndex)
	require.Less(t, spIndex+1, len(args))
	assert.Equal(t, "You are a helpful assistant", args[spIndex+1])
}

func TestBuildArgs_WithAllowedTools(t *testing.T) {
	req := ClaudeRequest{
		Prompt:       "Hello",
		AllowedTools: []string{"Read", "Edit", "Bash"},
	}
	args := buildArgs(req)

	assert.Contains(t, args, "--allowedTools")
	atIndex := indexOf(args, "--allowedTools")
	require.NotEqual(t, -1, atIndex)
	require.Less(t, atIndex+1, len(args))
	assert.Equal(t, "Read,Edit,Bash", args[atIndex+1])
}

func TestBuildArgs_WithContinue(t *testing.T) {
	req := ClaudeRequest{
		Prompt:   "Continue from here",
		Continue: true,
	}
	args := buildArgs(req)

	assert.Contains(t, args, "--continue")
}

func TestBuildArgs_WithExtraArgs(t *testing.T) {
	req := ClaudeRequest{
		Prompt:    "Hello",
		ExtraArgs: []string{"--verbose", "--debug"},
	}
	args := buildArgs(req)

	assert.Contains(t, args, "--verbose")
	assert.Contains(t, args, "--debug")
}

func TestBuildArgs_AllOptions(t *testing.T) {
	req := ClaudeRequest{
		Prompt:       "Do the task",
		SystemPrompt: "Be helpful",
		AllowedTools: []string{"Read"},
		Continue:     true,
		ExtraArgs:    []string{"--max-tokens", "1000"},
	}
	args := buildArgs(req)

	assert.Contains(t, args, "--output-format=stream-json")
	assert.Contains(t, args, "--system-prompt")
	assert.Contains(t, args, "--allowedTools")
	assert.Contains(t, args, "--continue")
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--max-tokens")
}

func TestGenerateLogFilename(t *testing.T) {
	filename := generateLogFilename("my-task-123")

	assert.True(t, strings.HasSuffix(filename, "-my-task-123.ndjson"))
	// Should start with a timestamp pattern (YYYYMMDD-HHMMSS)
	assert.Regexp(t, `^\d{8}-\d{6}-my-task-123\.ndjson$`, filename)
}

func TestGenerateLogFilename_SanitizesTaskID(t *testing.T) {
	// Task IDs with special characters should be sanitized
	filename := generateLogFilename("task/with/slashes")
	assert.NotContains(t, filename, "/")

	filename = generateLogFilename("task with spaces")
	assert.NotContains(t, filename, " ")
}

func TestSubprocessRunner_Implements_Runner(t *testing.T) {
	var _ Runner = (*SubprocessRunner)(nil)
}

func TestNewSubprocessRunner(t *testing.T) {
	logsDir := t.TempDir()
	runner := NewSubprocessRunner("claude", logsDir)

	assert.NotNil(t, runner)
	assert.Equal(t, "claude", runner.command)
	assert.Equal(t, logsDir, runner.logsDir)
}

func TestSubprocessRunner_Run_CommandNotFound(t *testing.T) {
	logsDir := t.TempDir()
	runner := NewSubprocessRunner("nonexistent-command-xyz", logsDir)

	req := ClaudeRequest{
		Cwd:    t.TempDir(),
		Prompt: "Hello",
	}

	ctx := context.Background()
	_, err := runner.Run(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-command-xyz")
}

func TestSubprocessRunner_Run_ContextCancellation(t *testing.T) {
	logsDir := t.TempDir()
	// Use a command that will take a while (sleep)
	runner := NewSubprocessRunner("sleep", logsDir)

	req := ClaudeRequest{
		Cwd:       t.TempDir(),
		Prompt:    "10", // sleep for 10 seconds
		ExtraArgs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runner.Run(ctx, req)
	elapsed := time.Since(start)

	require.Error(t, err)
	// Should have been cancelled quickly, not waited 10 seconds
	assert.Less(t, elapsed, 5*time.Second)
}

func TestSubprocessRunner_CreatesLogFile(t *testing.T) {
	logsDir := t.TempDir()
	// Use echo as a mock claude command
	runner := NewSubprocessRunner("echo", logsDir)

	req := ClaudeRequest{
		Cwd:    t.TempDir(),
		Prompt: `{"type":"result","subtype":"success","result":"done"}`,
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	// Note: echo doesn't produce valid NDJSON with result, so this will fail
	// but the log file should still be created
	_ = err
	_ = resp

	// Check that some log file was created
	entries, _ := os.ReadDir(logsDir)
	assert.NotEmpty(t, entries, "log file should be created even on failure")
}

func TestSubprocessRunner_SetsWorkingDirectory(t *testing.T) {
	logsDir := t.TempDir()
	workDir := t.TempDir()

	// Create a mock script that outputs the working directory in NDJSON
	mockScript := filepath.Join(t.TempDir(), "mock-pwd.sh")
	scriptContent := `#!/bin/bash
echo '{"type":"system","subtype":"init","session_id":"pwd-session","cwd":"'"$(pwd)"'"}'
echo '{"type":"result","subtype":"success","result":"'"$(pwd)"'"}'
`
	err := os.WriteFile(mockScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	runner := NewSubprocessRunner(mockScript, logsDir)

	req := ClaudeRequest{
		Cwd:    workDir,
		Prompt: "get working directory",
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	require.NoError(t, err)

	// The cwd from init and result should contain our working directory
	assert.Contains(t, resp.FinalText, workDir)
}

func TestSubprocessRunner_SetsEnvironment(t *testing.T) {
	logsDir := t.TempDir()
	workDir := t.TempDir()

	// Create a mock script that outputs an environment variable in NDJSON
	mockScript := filepath.Join(t.TempDir(), "mock-env.sh")
	scriptContent := `#!/bin/bash
echo '{"type":"system","subtype":"init","session_id":"env-session"}'
echo '{"type":"result","subtype":"success","result":"'"$TEST_VAR"'"}'
`
	err := os.WriteFile(mockScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	runner := NewSubprocessRunner(mockScript, logsDir)

	req := ClaudeRequest{
		Cwd:    workDir,
		Prompt: "get env var",
		Env: map[string]string{
			"TEST_VAR": "test_value_123",
		},
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	require.NoError(t, err)

	// The result should contain our environment variable value
	assert.Equal(t, "test_value_123", resp.FinalText)
}

// Helper function to find index of string in slice
func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

// Test with a mock Claude binary that outputs valid NDJSON
func TestSubprocessRunner_ParsesValidOutput(t *testing.T) {
	logsDir := t.TempDir()
	workDir := t.TempDir()

	// Create a mock script that outputs valid NDJSON
	mockScript := filepath.Join(workDir, "mock-claude.sh")
	scriptContent := `#!/bin/bash
echo '{"type":"system","subtype":"init","session_id":"test-session","model":"claude-test","cwd":"'"$PWD"'"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello from mock"}]}}'
echo '{"type":"result","subtype":"success","result":"Task completed successfully","total_cost_usd":0.001,"usage":{"input_tokens":100,"output_tokens":50}}'
`
	err := os.WriteFile(mockScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	runner := NewSubprocessRunner(mockScript, logsDir)

	req := ClaudeRequest{
		Cwd:    workDir,
		Prompt: "Do something",
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, "test-session", resp.SessionID)
	assert.Equal(t, "claude-test", resp.Model)
	assert.Equal(t, "Task completed successfully", resp.FinalText)
	assert.Equal(t, "Hello from mock", resp.StreamText)
	assert.Equal(t, 0.001, resp.TotalCostUSD)
	assert.Equal(t, 100, resp.Usage.InputTokens)
	assert.Equal(t, 50, resp.Usage.OutputTokens)
	assert.NotEmpty(t, resp.RawEventsPath)

	// Verify log file exists and contains the NDJSON
	_, err = os.Stat(resp.RawEventsPath)
	require.NoError(t, err)
}

func TestSubprocessRunner_ReturnsErrorResult(t *testing.T) {
	logsDir := t.TempDir()
	workDir := t.TempDir()

	// Create a mock script that outputs an error result
	mockScript := filepath.Join(workDir, "mock-claude-error.sh")
	scriptContent := `#!/bin/bash
echo '{"type":"system","subtype":"init","session_id":"error-session"}'
echo '{"type":"result","subtype":"error","is_error":true,"result":"Something went wrong"}'
`
	err := os.WriteFile(mockScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	runner := NewSubprocessRunner(mockScript, logsDir)

	req := ClaudeRequest{
		Cwd:    workDir,
		Prompt: "Do something that fails",
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	// Error result should still return successfully (the run completed)
	require.NoError(t, err)

	assert.Equal(t, "error-session", resp.SessionID)
	assert.Equal(t, "Something went wrong", resp.FinalText)
}

func TestSubprocessRunner_HandlesPermissionDenials(t *testing.T) {
	logsDir := t.TempDir()
	workDir := t.TempDir()

	// Create a mock script that outputs permission denials
	mockScript := filepath.Join(workDir, "mock-claude-perms.sh")
	scriptContent := `#!/bin/bash
echo '{"type":"system","subtype":"init","session_id":"perm-session"}'
echo '{"type":"result","subtype":"success","result":"Partial success","permission_denials":["edit /etc/passwd","run rm -rf /"]}'
`
	err := os.WriteFile(mockScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	runner := NewSubprocessRunner(mockScript, logsDir)

	req := ClaudeRequest{
		Cwd:    workDir,
		Prompt: "Do something restricted",
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.PermissionDenials, 2)
	assert.Equal(t, "edit /etc/passwd", resp.PermissionDenials[0])
	assert.Equal(t, "run rm -rf /", resp.PermissionDenials[1])
}

func TestSubprocessRunner_TaskIDInLogFilename(t *testing.T) {
	logsDir := t.TempDir()
	workDir := t.TempDir()

	// Create a mock script
	mockScript := filepath.Join(workDir, "mock-claude.sh")
	scriptContent := `#!/bin/bash
echo '{"type":"result","subtype":"success","result":"done"}'
`
	err := os.WriteFile(mockScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	runner := NewSubprocessRunner(mockScript, logsDir)
	runner.TaskID = "my-special-task"

	req := ClaudeRequest{
		Cwd:    workDir,
		Prompt: "Test",
	}

	ctx := context.Background()
	resp, err := runner.Run(ctx, req)
	require.NoError(t, err)

	assert.Contains(t, resp.RawEventsPath, "my-special-task")
}
