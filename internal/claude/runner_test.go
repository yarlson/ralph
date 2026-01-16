// Package claude provides integration with Claude Code subprocess execution.
package claude

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeRequest_Defaults(t *testing.T) {
	req := ClaudeRequest{
		Cwd:    "/workspace",
		Prompt: "hello",
	}

	assert.Equal(t, "/workspace", req.Cwd)
	assert.Equal(t, "hello", req.Prompt)
	assert.Empty(t, req.SystemPrompt)
	assert.Empty(t, req.AllowedTools)
	assert.False(t, req.Continue)
	assert.Empty(t, req.ExtraArgs)
	assert.Empty(t, req.Env)
}

func TestClaudeRequest_AllFields(t *testing.T) {
	req := ClaudeRequest{
		Cwd:          "/workspace/repo",
		SystemPrompt: "You are a helpful assistant",
		AllowedTools: []string{"Read", "Edit", "Bash"},
		Prompt:       "implement this feature",
		Continue:     true,
		ExtraArgs:    []string{"--verbose"},
		Env:          map[string]string{"DEBUG": "true"},
	}

	assert.Equal(t, "/workspace/repo", req.Cwd)
	assert.Equal(t, "You are a helpful assistant", req.SystemPrompt)
	assert.Equal(t, []string{"Read", "Edit", "Bash"}, req.AllowedTools)
	assert.Equal(t, "implement this feature", req.Prompt)
	assert.True(t, req.Continue)
	assert.Equal(t, []string{"--verbose"}, req.ExtraArgs)
	assert.Equal(t, map[string]string{"DEBUG": "true"}, req.Env)
}

func TestClaudeResponse_Defaults(t *testing.T) {
	resp := ClaudeResponse{}

	assert.Empty(t, resp.SessionID)
	assert.Empty(t, resp.Model)
	assert.Empty(t, resp.Version)
	assert.Empty(t, resp.FinalText)
	assert.Empty(t, resp.StreamText)
	assert.Equal(t, float64(0), resp.TotalCostUSD)
	assert.Nil(t, resp.PermissionDenials)
	assert.Empty(t, resp.RawEventsPath)
}

func TestClaudeResponse_AllFields(t *testing.T) {
	resp := ClaudeResponse{
		SessionID:    "abc123",
		Model:        "claude-3-opus",
		Version:      "2.1.9",
		FinalText:    "Task completed successfully",
		StreamText:   "Working on task...\nTask completed successfully",
		TotalCostUSD: 0.009631,
		Usage: ClaudeUsage{
			InputTokens:  1000,
			OutputTokens: 500,
		},
		PermissionDenials: []string{},
		RawEventsPath:     ".ralph/logs/claude/20260116-task.ndjson",
	}

	assert.Equal(t, "abc123", resp.SessionID)
	assert.Equal(t, "claude-3-opus", resp.Model)
	assert.Equal(t, "2.1.9", resp.Version)
	assert.Equal(t, "Task completed successfully", resp.FinalText)
	assert.Equal(t, "Working on task...\nTask completed successfully", resp.StreamText)
	assert.Equal(t, 0.009631, resp.TotalCostUSD)
	assert.Equal(t, 1000, resp.Usage.InputTokens)
	assert.Equal(t, 500, resp.Usage.OutputTokens)
	assert.Empty(t, resp.PermissionDenials)
	assert.Equal(t, ".ralph/logs/claude/20260116-task.ndjson", resp.RawEventsPath)
}

func TestClaudeUsage_Fields(t *testing.T) {
	usage := ClaudeUsage{
		InputTokens:       1234,
		OutputTokens:      5678,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
	}

	assert.Equal(t, 1234, usage.InputTokens)
	assert.Equal(t, 5678, usage.OutputTokens)
	assert.Equal(t, 100, usage.CacheCreationTokens)
	assert.Equal(t, 200, usage.CacheReadTokens)
}

func TestClaudeRequest_JSONSerialization(t *testing.T) {
	req := ClaudeRequest{
		Cwd:          "/workspace",
		SystemPrompt: "test prompt",
		AllowedTools: []string{"Read", "Edit"},
		Prompt:       "hello",
		Continue:     true,
		ExtraArgs:    []string{"--flag"},
		Env:          map[string]string{"KEY": "value"},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded ClaudeRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.Cwd, decoded.Cwd)
	assert.Equal(t, req.SystemPrompt, decoded.SystemPrompt)
	assert.Equal(t, req.AllowedTools, decoded.AllowedTools)
	assert.Equal(t, req.Prompt, decoded.Prompt)
	assert.Equal(t, req.Continue, decoded.Continue)
	assert.Equal(t, req.ExtraArgs, decoded.ExtraArgs)
	assert.Equal(t, req.Env, decoded.Env)
}

func TestClaudeResponse_JSONSerialization(t *testing.T) {
	resp := ClaudeResponse{
		SessionID:    "session-123",
		Model:        "claude-opus",
		Version:      "2.1.9",
		FinalText:    "done",
		StreamText:   "working...\ndone",
		TotalCostUSD: 1.23,
		Usage: ClaudeUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
		PermissionDenials: []string{"write_access"},
		RawEventsPath:     "/path/to/log.ndjson",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded ClaudeResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.SessionID, decoded.SessionID)
	assert.Equal(t, resp.Model, decoded.Model)
	assert.Equal(t, resp.Version, decoded.Version)
	assert.Equal(t, resp.FinalText, decoded.FinalText)
	assert.Equal(t, resp.StreamText, decoded.StreamText)
	assert.Equal(t, resp.TotalCostUSD, decoded.TotalCostUSD)
	assert.Equal(t, resp.Usage.InputTokens, decoded.Usage.InputTokens)
	assert.Equal(t, resp.Usage.OutputTokens, decoded.Usage.OutputTokens)
	assert.Equal(t, resp.PermissionDenials, decoded.PermissionDenials)
	assert.Equal(t, resp.RawEventsPath, decoded.RawEventsPath)
}

func TestClaudeUsage_JSONSerialization(t *testing.T) {
	usage := ClaudeUsage{
		InputTokens:       1000,
		OutputTokens:      500,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var decoded ClaudeUsage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, usage.InputTokens, decoded.InputTokens)
	assert.Equal(t, usage.OutputTokens, decoded.OutputTokens)
	assert.Equal(t, usage.CacheCreationTokens, decoded.CacheCreationTokens)
	assert.Equal(t, usage.CacheReadTokens, decoded.CacheReadTokens)
}

func TestRunner_InterfaceExists(t *testing.T) {
	// Verify that Runner interface is properly defined
	// This test ensures the interface can be implemented
	var _ Runner = (*mockRunner)(nil)
}

// mockRunner is a test implementation of Runner interface
type mockRunner struct {
	runFunc func(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error)
}

func (m *mockRunner) Run(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, req)
	}
	return &ClaudeResponse{}, nil
}

func TestRunner_MockImplementation(t *testing.T) {
	expectedResp := &ClaudeResponse{
		SessionID: "test-session",
		FinalText: "test result",
	}

	mock := &mockRunner{
		runFunc: func(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error) {
			assert.Equal(t, "/workspace", req.Cwd)
			assert.Equal(t, "test prompt", req.Prompt)
			return expectedResp, nil
		},
	}

	var runner Runner = mock
	resp, err := runner.Run(context.Background(), ClaudeRequest{
		Cwd:    "/workspace",
		Prompt: "test prompt",
	})

	require.NoError(t, err)
	assert.Equal(t, expectedResp.SessionID, resp.SessionID)
	assert.Equal(t, expectedResp.FinalText, resp.FinalText)
}
