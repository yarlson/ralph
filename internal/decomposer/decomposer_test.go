package decomposer

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/claude"
)

// mockRunner is a mock implementation of claude.Runner for testing
type mockRunner struct {
	response *claude.ClaudeResponse
	err      error
}

func (m *mockRunner) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	return m.response, m.err
}

func TestExtractYAMLContent_FromCodeBlock(t *testing.T) {
	resp := &claude.ClaudeResponse{
		FinalText: "Here's the generated YAML:\n\n```yaml\ntasks:\n  - id: test-1\n    title: Test Task\n```\n\nDone!",
	}

	yaml := extractYAMLContent(resp)
	assert.NotEmpty(t, yaml)
	assert.Contains(t, yaml, "tasks:")
	assert.Contains(t, yaml, "id: test-1")
	assert.NotContains(t, yaml, "```")
}

func TestExtractYAMLContent_FromPlainText(t *testing.T) {
	resp := &claude.ClaudeResponse{
		FinalText: "tasks:\n  - id: test-1\n    title: Test Task\n    parentId: root",
	}

	yaml := extractYAMLContent(resp)
	assert.NotEmpty(t, yaml)
	assert.Contains(t, yaml, "tasks:")
	assert.Contains(t, yaml, "id: test-1")
}

func TestExtractYAMLContent_WithYAMLDocument(t *testing.T) {
	resp := &claude.ClaudeResponse{
		FinalText: "---\ntasks:\n  - id: test-1\n    title: Test Task",
	}

	yaml := extractYAMLContent(resp)
	assert.NotEmpty(t, yaml)
	assert.Contains(t, yaml, "tasks:")
}

func TestExtractYAMLContent_EmptyResponse(t *testing.T) {
	resp := &claude.ClaudeResponse{
		FinalText: "No YAML here, just text",
	}

	yaml := extractYAMLContent(resp)
	assert.Empty(t, yaml)
}

func TestExtractYAMLContent_FallbackToStreamText(t *testing.T) {
	resp := &claude.ClaudeResponse{
		FinalText:  "",
		StreamText: "```yaml\ntasks:\n  - id: test-1\n```",
	}

	yaml := extractYAMLContent(resp)
	assert.NotEmpty(t, yaml)
	assert.Contains(t, yaml, "tasks:")
}

func TestNewDecomposer_WithMockRunner(t *testing.T) {
	mockResp := &claude.ClaudeResponse{
		SessionID:    "test-session-123",
		Model:        "claude-sonnet-4",
		FinalText:    "```yaml\ntasks:\n  - id: test-root\n    title: Test Feature\n```",
		TotalCostUSD: 0.05,
	}

	runner := &mockRunner{response: mockResp}
	dec := NewDecomposer(runner)

	// Verify the decomposer is created correctly
	assert.NotNil(t, dec)
	assert.NotNil(t, dec.runner)
}

func TestDecompose_ExtractYAMLFromResponse(t *testing.T) {
	tests := []struct {
		name          string
		response      *claude.ClaudeResponse
		expectError   bool
		expectContent bool
	}{
		{
			name: "YAML in code block",
			response: &claude.ClaudeResponse{
				FinalText: "```yaml\ntasks:\n  - id: root\n```",
			},
			expectError:   false,
			expectContent: true,
		},
		{
			name: "plain YAML",
			response: &claude.ClaudeResponse{
				FinalText: "tasks:\n  - id: root",
			},
			expectError:   false,
			expectContent: true,
		},
		{
			name: "no YAML found",
			response: &claude.ClaudeResponse{
				FinalText: "Sorry, I couldn't generate the YAML.",
			},
			expectError:   false,
			expectContent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := extractYAMLContent(tt.response)
			if tt.expectContent {
				assert.NotEmpty(t, yaml)
				assert.Contains(t, yaml, "tasks:")
			} else {
				assert.Empty(t, yaml)
			}
		})
	}
}

func TestNewDecomposer(t *testing.T) {
	runner := &mockRunner{}
	dec := NewDecomposer(runner)

	require.NotNil(t, dec)
	assert.Equal(t, runner, dec.runner)
}

func TestSystemPrompt_Contains_RequiredElements(t *testing.T) {
	// Verify the system prompt contains key elements
	assert.Contains(t, systemPrompt, "Task Decomposer")
	assert.Contains(t, systemPrompt, "YAML")
	assert.Contains(t, systemPrompt, "tasks:")
	assert.Contains(t, systemPrompt, "id:")
	assert.Contains(t, systemPrompt, "title:")
	assert.Contains(t, systemPrompt, "parentId:")
	assert.Contains(t, systemPrompt, "dependsOn:")
	assert.Contains(t, systemPrompt, "acceptance:")
	assert.Contains(t, systemPrompt, "verify:")
	assert.Contains(t, systemPrompt, "labels:")
	assert.Contains(t, systemPrompt, "DRY")
	assert.Contains(t, systemPrompt, "KISS")
	assert.Contains(t, systemPrompt, "YAGNI")
}

func TestSystemPrompt_ContainsTaskModelRules(t *testing.T) {
	// Verify task model rules are present
	assert.Contains(t, systemPrompt, "ONE root task")
	assert.Contains(t, systemPrompt, "epic")
	assert.Contains(t, systemPrompt, "leaf tasks")
	assert.Contains(t, systemPrompt, "kebab-case")
}

func TestSystemPrompt_ContainsMappingRules(t *testing.T) {
	// Verify mapping rules are present
	assert.Contains(t, systemPrompt, "Requirements")
	assert.Contains(t, systemPrompt, "User Journeys")
	assert.Contains(t, systemPrompt, "Analytics")
	assert.Contains(t, systemPrompt, "Risks")
	assert.Contains(t, systemPrompt, "Rollout")
	assert.Contains(t, systemPrompt, "Non-goals")
}

func TestExtractYAMLContent_CodeBlockVariants(t *testing.T) {
	tests := []struct {
		name     string
		response string
		contains string
	}{
		{
			name:     "yaml code block",
			response: "```yaml\ntasks:\n  - id: test\n```",
			contains: "tasks:",
		},
		{
			name:     "yml code block",
			response: "```yml\ntasks:\n  - id: test\n```",
			contains: "tasks:",
		},
		{
			name:     "no language specifier",
			response: "```\ntasks:\n  - id: test\n```",
			contains: "tasks:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &claude.ClaudeResponse{FinalText: tt.response}
			yaml := extractYAMLContent(resp)
			assert.Contains(t, yaml, tt.contains)
			assert.False(t, strings.Contains(yaml, "```"))
		})
	}
}
