package prompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func TestBuildRetryPrompt_NilTask(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: nil,
	}

	_, err := builder.BuildRetryPrompt(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task is required")
}

func TestBuildRetryPrompt_MinimalContext(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, prompt)

	// Should contain task info
	assert.Contains(t, prompt, "Test Task")
	assert.Contains(t, prompt, "A test task")

	// Should contain fix-only directive
	assert.Contains(t, prompt, "fix")
}

func TestBuildRetryPrompt_WithFailureOutput(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		FailureOutput: "Error: undefined variable 'foo'\n  at line 42",
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should contain failure output
	assert.Contains(t, prompt, "undefined variable 'foo'")
	assert.Contains(t, prompt, "line 42")

	// Should have a section header for failures
	assert.Contains(t, prompt, "Verification Failed")
}

func TestBuildRetryPrompt_WithFailureSignature(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		FailureSignature: "abc123def",
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should contain failure signature for debugging context
	assert.Contains(t, prompt, "abc123def")
}

func TestBuildRetryPrompt_WithUserFeedback(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		UserFeedback: "Please focus on the authentication module",
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should contain user feedback
	assert.Contains(t, prompt, "authentication module")

	// Should have a section header for feedback
	assert.Contains(t, prompt, "User Feedback")
}

func TestBuildRetryPrompt_WithAllContext(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Fix Authentication Bug",
			Description: "Fix the login validation",
			Acceptance: []string{
				"Users can log in",
				"Invalid credentials show error",
			},
			Verify: [][]string{
				{"go", "test", "./..."},
			},
		},
		FailureOutput:    "FAIL: TestLogin\nexpected true, got false",
		FailureSignature: "sig123",
		UserFeedback:     "Check the password hash comparison",
		AttemptNumber:    3,
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Task info
	assert.Contains(t, prompt, "Fix Authentication Bug")
	assert.Contains(t, prompt, "login validation")

	// Acceptance criteria
	assert.Contains(t, prompt, "Users can log in")
	assert.Contains(t, prompt, "Invalid credentials show error")

	// Verification commands
	assert.Contains(t, prompt, "go test ./...")

	// Failure output
	assert.Contains(t, prompt, "FAIL: TestLogin")
	assert.Contains(t, prompt, "expected true, got false")

	// User feedback
	assert.Contains(t, prompt, "password hash comparison")

	// Attempt number
	assert.Contains(t, prompt, "3")
}

func TestBuildRetryPrompt_FixOnlyDirective(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		FailureOutput: "Some error",
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should contain strong fix-only directive
	lower := strings.ToLower(prompt)
	assert.Contains(t, lower, "fix")

	// Should emphasize not adding new features
	assert.True(t,
		strings.Contains(lower, "only") ||
			strings.Contains(lower, "do not add") ||
			strings.Contains(lower, "minimal"),
		"should emphasize fix-only approach")
}

func TestBuildRetryPrompt_TruncatesLargeFailureOutput(t *testing.T) {
	opts := &SizeOptions{
		MaxFailureBytes: 100,
	}
	builder := NewBuilder(opts)

	// Create a large failure output
	largeOutput := strings.Repeat("error line\n", 100)

	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		FailureOutput: largeOutput,
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should truncate the failure output
	assert.True(t, len(prompt) < len(largeOutput)+500,
		"prompt should be much smaller than original large output")

	// Should have truncation marker
	assert.Contains(t, prompt, "truncated")
}

func TestBuildRetryPrompt_AttemptNumber(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		AttemptNumber: 5,
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should show attempt number
	assert.Contains(t, prompt, "5")
	assert.Contains(t, prompt, "attempt")
}

func TestBuildRetryPrompt_FirstAttempt(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:          "test-task",
			Title:       "Test Task",
			Description: "A test task",
		},
		AttemptNumber: 0, // Zero means not set
	}

	prompt, err := builder.BuildRetryPrompt(ctx)
	require.NoError(t, err)

	// Should not contain "attempt 0"
	assert.NotContains(t, prompt, "attempt 0")
	assert.NotContains(t, prompt, "Attempt 0")
}

func TestRetryContext_Fields(t *testing.T) {
	ctx := RetryContext{
		Task: &taskstore.Task{
			ID:    "task-1",
			Title: "Task 1",
		},
		FailureOutput:    "error",
		FailureSignature: "sig123",
		UserFeedback:     "feedback",
		AttemptNumber:    2,
	}

	assert.Equal(t, "task-1", ctx.Task.ID)
	assert.Equal(t, "error", ctx.FailureOutput)
	assert.Equal(t, "sig123", ctx.FailureSignature)
	assert.Equal(t, "feedback", ctx.UserFeedback)
	assert.Equal(t, 2, ctx.AttemptNumber)
}

func TestBuildRetrySystemPrompt(t *testing.T) {
	builder := NewBuilder(nil)
	prompt := builder.BuildRetrySystemPrompt()

	// Should contain retry-specific instructions
	assert.Contains(t, prompt, "retry")

	// Should contain fix-only directive
	assert.Contains(t, strings.ToLower(prompt), "fix")

	// Should emphasize analyzing failures
	lower := strings.ToLower(prompt)
	assert.True(t,
		strings.Contains(lower, "fail") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "verification"),
		"should mention failures")
}

func TestBuildRetrySystemPrompt_DiffersFromRegular(t *testing.T) {
	builder := NewBuilder(nil)

	regularPrompt := builder.BuildSystemPrompt()
	retryPrompt := builder.BuildRetrySystemPrompt()

	// They should be different
	assert.NotEqual(t, regularPrompt, retryPrompt)

	// Retry prompt should be more focused
	assert.Contains(t, strings.ToLower(retryPrompt), "retry")
}
