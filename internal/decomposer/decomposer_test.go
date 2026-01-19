package decomposer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/ralph/internal/claude"
	"github.com/yarlson/ralph/internal/config"
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
	assert.Contains(t, getSystemPrompt(), "Task Decomposer")
	assert.Contains(t, getSystemPrompt(), "YAML")
	assert.Contains(t, getSystemPrompt(), "tasks:")
	assert.Contains(t, getSystemPrompt(), "id:")
	assert.Contains(t, getSystemPrompt(), "title:")
	assert.Contains(t, getSystemPrompt(), "parentId:")
	assert.Contains(t, getSystemPrompt(), "dependsOn:")
	assert.Contains(t, getSystemPrompt(), "acceptance:")
	assert.Contains(t, getSystemPrompt(), "verify:")
	assert.Contains(t, getSystemPrompt(), "labels:")
	assert.Contains(t, getSystemPrompt(), "DRY")
	assert.Contains(t, getSystemPrompt(), "KISS")
	assert.Contains(t, getSystemPrompt(), "YAGNI")
}

func TestSystemPrompt_ContainsTaskModelRules(t *testing.T) {
	// Verify task model rules are present
	assert.Contains(t, getSystemPrompt(), "ONE root task")
	assert.Contains(t, getSystemPrompt(), "epic")
	assert.Contains(t, getSystemPrompt(), "leaf tasks")
	assert.Contains(t, getSystemPrompt(), "kebab-case")
}

func TestSystemPrompt_ContainsMappingRules(t *testing.T) {
	// Verify mapping rules are present
	assert.Contains(t, getSystemPrompt(), "Requirements")
	assert.Contains(t, getSystemPrompt(), "User Journeys")
	assert.Contains(t, getSystemPrompt(), "Analytics")
	assert.Contains(t, getSystemPrompt(), "Risks")
	assert.Contains(t, getSystemPrompt(), "Rollout")
	assert.Contains(t, getSystemPrompt(), "Non-goals")
}

func TestSystemPrompt_ContainsForbiddenTestPatterns(t *testing.T) {
	// Verify forbidden test/validation patterns are documented
	assert.Contains(t, getSystemPrompt(), "Test-only tasks")
	assert.Contains(t, getSystemPrompt(), "Validation-only tasks")
	assert.Contains(t, getSystemPrompt(), "Tests are PART of implementation, not separate tasks")
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

// validTaskYAML is a valid YAML string for testing that passes all linter checks.
const validTaskYAML = `tasks:
  - id: test-root
    title: Test Root
    description: This is the root task
    status: open
    acceptance:
      - Root task exists
    verify:
      - ["go", "test", "./..."]
`

// invalidTaskYAML has an invalid parent reference.
const invalidTaskYAML = `tasks:
  - id: test-child
    title: Test Child
    description: This is a child task
    parentId: non-existent-parent
    status: open
    acceptance:
      - Child task exists
    verify:
      - ["go", "test", "./..."]
`

// fixedTaskYAML is the corrected version after retry.
const fixedTaskYAML = `tasks:
  - id: test-root
    title: Test Root
    description: This is the root task
    status: open
    acceptance:
      - Root task exists
    verify:
      - ["go", "test", "./..."]
  - id: test-child
    title: Test Child
    description: This is a child task
    parentId: test-root
    status: open
    acceptance:
      - Child task exists
    verify:
      - ["go", "test", "./..."]
`

// sequentialMockRunner returns different responses for each call.
type sequentialMockRunner struct {
	responses []*claude.ClaudeResponse
	errors    []error
	callCount int
}

func (m *sequentialMockRunner) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	if m.callCount >= len(m.responses) {
		return nil, nil
	}
	resp := m.responses[m.callCount]
	var err error
	if m.callCount < len(m.errors) {
		err = m.errors[m.callCount]
	}
	m.callCount++
	return resp, err
}

func TestValidateAndRetry_Success(t *testing.T) {
	// Test case where YAML is valid on first attempt
	runner := &mockRunner{}
	dec := NewDecomposer(runner)

	ctx := context.Background()
	prdContent := "# Test PRD\nThis is a test PRD."

	result, err := dec.validateAndRetry(ctx, prdContent, validTaskYAML)

	require.NoError(t, err)
	assert.Equal(t, validTaskYAML, result)
}

func TestValidateAndRetry_RetryOnError(t *testing.T) {
	// Test case where YAML fails initially but succeeds after retry
	runner := &sequentialMockRunner{
		responses: []*claude.ClaudeResponse{
			{FinalText: fixedTaskYAML},
		},
		errors: []error{nil},
	}
	dec := NewDecomposer(runner)

	ctx := context.Background()
	prdContent := "# Test PRD\nThis is a test PRD."

	result, err := dec.validateAndRetry(ctx, prdContent, invalidTaskYAML)

	require.NoError(t, err)
	assert.Contains(t, result, "test-root")
	assert.Contains(t, result, "test-child")
}

func TestValidateAndRetry_MaxRetriesExceeded(t *testing.T) {
	// Test case where YAML keeps failing after max retries
	runner := &sequentialMockRunner{
		responses: []*claude.ClaudeResponse{
			{FinalText: invalidTaskYAML},
			{FinalText: invalidTaskYAML},
		},
		errors: []error{nil, nil},
	}
	dec := NewDecomposer(runner)

	ctx := context.Background()
	prdContent := "# Test PRD\nThis is a test PRD."

	_, err := dec.validateAndRetry(ctx, prdContent, invalidTaskYAML)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed after")
}

func TestValidateAndRetry_InvalidYAMLSyntax(t *testing.T) {
	// Test case where YAML has invalid syntax
	runner := &sequentialMockRunner{
		responses: []*claude.ClaudeResponse{
			{FinalText: validTaskYAML},
		},
		errors: []error{nil},
	}
	dec := NewDecomposer(runner)

	ctx := context.Background()
	prdContent := "# Test PRD\nThis is a test PRD."
	invalidSyntax := "tasks:\n  - id: test\n    title: [invalid yaml"

	result, err := dec.validateAndRetry(ctx, prdContent, invalidSyntax)

	require.NoError(t, err)
	assert.Contains(t, result, "id: test-root")
	assert.Contains(t, result, "title: Test Root")
}

func TestValidateAndRetry_ClaudeError(t *testing.T) {
	// Test case where Claude returns an error during retry
	runner := &sequentialMockRunner{
		responses: []*claude.ClaudeResponse{nil},
		errors:    []error{assert.AnError},
	}
	dec := NewDecomposer(runner)

	ctx := context.Background()
	prdContent := "# Test PRD\nThis is a test PRD."

	_, err := dec.validateAndRetry(ctx, prdContent, invalidTaskYAML)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get fixed YAML from Claude")
}

func TestMaxValidationRetries_Value(t *testing.T) {
	assert.Equal(t, 2, maxValidationRetries)
}

func TestDecompose_WritesToDefaultPath(t *testing.T) {
	// Create a temporary directory to simulate the work directory
	tmpDir := t.TempDir()

	// Create a PRD file
	prdPath := filepath.Join(tmpDir, "PRD.md")
	prdContent := "# Test PRD\nThis is a test PRD."
	require.NoError(t, os.WriteFile(prdPath, []byte(prdContent), 0644))

	// Create a mock runner that returns valid YAML
	runner := &mockRunner{
		response: &claude.ClaudeResponse{
			SessionID:    "test-session-123",
			Model:        "claude-sonnet-4",
			FinalText:    validTaskYAML,
			TotalCostUSD: 0.05,
		},
	}

	dec := NewDecomposer(runner)

	ctx := context.Background()
	result, err := dec.Decompose(ctx, DecomposeRequest{
		PRDPath: prdPath,
		WorkDir: tmpDir,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify OutputPath is set correctly
	expectedPath := filepath.Join(tmpDir, config.DefaultTasksFile)
	assert.Equal(t, expectedPath, result.OutputPath)

	// Verify the directory was created
	tasksDir := filepath.Dir(expectedPath)
	info, err := os.Stat(tasksDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify the file was written
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	// Compare trimmed content since YAML extraction trims the content
	assert.Equal(t, strings.TrimSpace(validTaskYAML), strings.TrimSpace(string(content)))
}

// TestDecompose_ValidYAMLSucceeds tests that valid YAML passes on first attempt
// without triggering any retry logic.
func TestDecompose_ValidYAMLSucceeds(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD file
	prdPath := filepath.Join(tmpDir, "PRD.md")
	prdContent := "# Test PRD\nThis is a test PRD for valid YAML test."
	require.NoError(t, os.WriteFile(prdPath, []byte(prdContent), 0644))

	// Mock runner returns valid YAML on first call
	runner := &mockRunner{
		response: &claude.ClaudeResponse{
			SessionID:    "valid-session",
			Model:        "claude-sonnet-4",
			FinalText:    validTaskYAML,
			TotalCostUSD: 0.01,
		},
	}

	dec := NewDecomposer(runner)

	ctx := context.Background()
	result, err := dec.Decompose(ctx, DecomposeRequest{
		PRDPath: prdPath,
		WorkDir: tmpDir,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "valid-session", result.SessionID)
	assert.Contains(t, result.YAMLContent, "test-root")
	assert.Contains(t, result.YAMLContent, "Test Root")

	// Verify the file was written
	content, err := os.ReadFile(result.OutputPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-root")
}

// TestDecompose_InvalidYAMLRetries tests that invalid YAML triggers retry
// and succeeds after Claude fixes it.
func TestDecompose_InvalidYAMLRetries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD file
	prdPath := filepath.Join(tmpDir, "PRD.md")
	prdContent := "# Test PRD\nThis is a test PRD for retry test."
	require.NoError(t, os.WriteFile(prdPath, []byte(prdContent), 0644))

	// Sequential runner: first returns invalid YAML, then returns fixed YAML on retry
	runner := &sequentialMockRunner{
		responses: []*claude.ClaudeResponse{
			// First call: Decompose returns invalid YAML (orphan child)
			{
				SessionID:    "initial-session",
				Model:        "claude-sonnet-4",
				FinalText:    invalidTaskYAML,
				TotalCostUSD: 0.01,
			},
			// Second call: askClaudeToFix returns corrected YAML
			{
				SessionID:    "fix-session",
				Model:        "claude-sonnet-4",
				FinalText:    fixedTaskYAML,
				TotalCostUSD: 0.02,
			},
		},
		errors: []error{nil, nil},
	}

	dec := NewDecomposer(runner)

	ctx := context.Background()
	result, err := dec.Decompose(ctx, DecomposeRequest{
		PRDPath: prdPath,
		WorkDir: tmpDir,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify the fixed YAML is returned
	assert.Contains(t, result.YAMLContent, "test-root")
	assert.Contains(t, result.YAMLContent, "test-child")
	assert.Contains(t, result.YAMLContent, "parentId: test-root")

	// Verify both calls were made (initial + fix)
	assert.Equal(t, 2, runner.callCount)
}

// TestDecompose_MaxRetriesExceeded tests that decomposition fails after max retries
// when Claude cannot produce valid YAML.
func TestDecompose_MaxRetriesExceeded(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD file
	prdPath := filepath.Join(tmpDir, "PRD.md")
	prdContent := "# Test PRD\nThis is a test PRD for max retries test."
	require.NoError(t, os.WriteFile(prdPath, []byte(prdContent), 0644))

	// Sequential runner: always returns invalid YAML, exhausting retries
	runner := &sequentialMockRunner{
		responses: []*claude.ClaudeResponse{
			// First call: Decompose returns invalid YAML
			{
				SessionID: "initial-session",
				FinalText: invalidTaskYAML,
			},
			// Second call: First retry still returns invalid YAML
			{
				SessionID: "retry-1-session",
				FinalText: invalidTaskYAML,
			},
			// Third call: Second retry still returns invalid YAML
			{
				SessionID: "retry-2-session",
				FinalText: invalidTaskYAML,
			},
		},
		errors: []error{nil, nil, nil},
	}

	dec := NewDecomposer(runner)

	ctx := context.Background()
	_, err := dec.Decompose(ctx, DecomposeRequest{
		PRDPath: prdPath,
		WorkDir: tmpDir,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "retries")

	// Verify all retry attempts were made (initial + maxValidationRetries fix attempts)
	assert.Equal(t, maxValidationRetries+1, runner.callCount)
}

// capturingMockRunner captures request details for inspection.
type capturingMockRunner struct {
	responses []*claude.ClaudeResponse
	errors    []error
	callCount int
	requests  []claude.ClaudeRequest
}

func (m *capturingMockRunner) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	m.requests = append(m.requests, req)
	if m.callCount >= len(m.responses) {
		return nil, nil
	}
	resp := m.responses[m.callCount]
	var err error
	if m.callCount < len(m.errors) {
		err = m.errors[m.callCount]
	}
	m.callCount++
	return resp, err
}

// TestDecompose_FixPromptContainsContext tests that the fix prompt sent to Claude
// includes the original PRD content and the validation errors.
func TestDecompose_FixPromptContainsContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PRD file with distinctive content
	prdPath := filepath.Join(tmpDir, "PRD.md")
	prdContent := "# Unique PRD Title\nThis PRD has unique content for testing fix prompt context."
	require.NoError(t, os.WriteFile(prdPath, []byte(prdContent), 0644))

	// Capturing runner: first returns invalid YAML, then returns fixed YAML
	runner := &capturingMockRunner{
		responses: []*claude.ClaudeResponse{
			// First call: Decompose returns invalid YAML
			{
				SessionID: "initial-session",
				FinalText: invalidTaskYAML,
			},
			// Second call: askClaudeToFix returns corrected YAML
			{
				SessionID: "fix-session",
				FinalText: fixedTaskYAML,
			},
		},
		errors: []error{nil, nil},
	}

	dec := NewDecomposer(runner)

	ctx := context.Background()
	_, err := dec.Decompose(ctx, DecomposeRequest{
		PRDPath: prdPath,
		WorkDir: tmpDir,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(runner.requests), "expected 2 requests: initial + fix")

	// Inspect the fix request (second call)
	fixRequest := runner.requests[1]

	// Verify fix prompt contains PRD content
	assert.Contains(t, fixRequest.Prompt, "Unique PRD Title", "fix prompt should contain PRD title")
	assert.Contains(t, fixRequest.Prompt, "unique content for testing", "fix prompt should contain PRD content")

	// Verify fix prompt contains the failed YAML
	assert.Contains(t, fixRequest.Prompt, "test-child", "fix prompt should contain failed YAML")
	assert.Contains(t, fixRequest.Prompt, "non-existent-parent", "fix prompt should contain invalid parent reference")

	// Verify fix prompt contains validation error context
	// The invalid YAML has orphan parent reference, so error should mention parent
	assert.Contains(t, fixRequest.Prompt, "Validation Errors", "fix prompt should have validation errors section")
}
