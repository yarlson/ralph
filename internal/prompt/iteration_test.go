// Package prompt provides prompt packaging for Claude Code iterations.
package prompt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func TestIterationContext_Defaults(t *testing.T) {
	ctx := IterationContext{}

	assert.Nil(t, ctx.Task)
	assert.Empty(t, ctx.CodebasePatterns)
	assert.Empty(t, ctx.DiffStat)
	assert.Empty(t, ctx.ChangedFiles)
	assert.Empty(t, ctx.FailureOutput)
	assert.Empty(t, ctx.UserFeedback)
	assert.False(t, ctx.IsRetry)
}

func TestIterationContext_AllFields(t *testing.T) {
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "A test task",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{
		Task:             task,
		CodebasePatterns: "- Pattern 1\n- Pattern 2",
		DiffStat:         "1 file changed, 10 insertions(+)",
		ChangedFiles:     []string{"file1.go", "file2.go"},
		FailureOutput:    "test failed",
		UserFeedback:     "please fix the bug",
		IsRetry:          true,
	}

	assert.Equal(t, task, ctx.Task)
	assert.Equal(t, "- Pattern 1\n- Pattern 2", ctx.CodebasePatterns)
	assert.Equal(t, "1 file changed, 10 insertions(+)", ctx.DiffStat)
	assert.Equal(t, []string{"file1.go", "file2.go"}, ctx.ChangedFiles)
	assert.Equal(t, "test failed", ctx.FailureOutput)
	assert.Equal(t, "please fix the bug", ctx.UserFeedback)
	assert.True(t, ctx.IsRetry)
}

func TestSizeOptions_Defaults(t *testing.T) {
	opts := DefaultSizeOptions()

	assert.Equal(t, 8000, opts.MaxPromptBytes)
	assert.Equal(t, 2000, opts.MaxPatternsBytes)
	assert.Equal(t, 1000, opts.MaxDiffBytes)
	assert.Equal(t, 2000, opts.MaxFailureBytes)
}

func TestSizeOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    SizeOptions
		wantErr bool
	}{
		{
			name:    "valid options",
			opts:    DefaultSizeOptions(),
			wantErr: false,
		},
		{
			name:    "zero values allowed",
			opts:    SizeOptions{},
			wantErr: false,
		},
		{
			name:    "negative max prompt bytes",
			opts:    SizeOptions{MaxPromptBytes: -1},
			wantErr: true,
		},
		{
			name:    "negative max patterns bytes",
			opts:    SizeOptions{MaxPatternsBytes: -1},
			wantErr: true,
		},
		{
			name:    "negative max diff bytes",
			opts:    SizeOptions{MaxDiffBytes: -1},
			wantErr: true,
		},
		{
			name:    "negative max failure bytes",
			opts:    SizeOptions{MaxFailureBytes: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuilderNew(t *testing.T) {
	builder := NewBuilder(nil)

	assert.NotNil(t, builder)
	assert.Equal(t, DefaultSizeOptions(), builder.opts)
}

func TestBuilderNewWithOptions(t *testing.T) {
	opts := &SizeOptions{
		MaxPromptBytes:   10000,
		MaxPatternsBytes: 3000,
	}
	builder := NewBuilder(opts)

	assert.NotNil(t, builder)
	assert.Equal(t, 10000, builder.opts.MaxPromptBytes)
	assert.Equal(t, 3000, builder.opts.MaxPatternsBytes)
}

func TestBuilderBuildSystemPrompt(t *testing.T) {
	builder := NewBuilder(nil)
	systemPrompt := builder.BuildSystemPrompt()

	assert.Contains(t, systemPrompt, "Ralph harness")
	assert.Contains(t, systemPrompt, "verification")
	assert.Contains(t, systemPrompt, "commit")
}

func TestBuilderBuildUserPrompt_MinimalTask(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "minimal-task",
		Title:       "Minimal Task",
		Description: "A minimal task description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{Task: task}
	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	assert.Contains(t, prompt, "Minimal Task")
	assert.Contains(t, prompt, "A minimal task description")
}

func TestBuilderBuildUserPrompt_FullTask(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "full-task",
		Title:       "Full Task",
		Description: "A full task description with details",
		Status:      taskstore.StatusOpen,
		Acceptance: []string{
			"Acceptance criterion 1",
			"Acceptance criterion 2",
		},
		Verify: [][]string{
			{"go", "test", "./..."},
			{"go", "build", "./..."},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := IterationContext{
		Task:             task,
		CodebasePatterns: "- Use interfaces on consumers\n- Return concrete types",
		DiffStat:         "2 files changed, 50 insertions(+), 10 deletions(-)",
		ChangedFiles:     []string{"internal/foo/bar.go", "internal/foo/bar_test.go"},
	}

	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	assert.Contains(t, prompt, "Full Task")
	assert.Contains(t, prompt, "A full task description with details")
	assert.Contains(t, prompt, "Acceptance criterion 1")
	assert.Contains(t, prompt, "Acceptance criterion 2")
	assert.Contains(t, prompt, "go test ./...")
	assert.Contains(t, prompt, "go build ./...")
	assert.Contains(t, prompt, "Use interfaces on consumers")
	assert.Contains(t, prompt, "Return concrete types")
	assert.Contains(t, prompt, "2 files changed")
	assert.Contains(t, prompt, "internal/foo/bar.go")
}

func TestBuilderBuildUserPrompt_WithInstructions(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{Task: task}
	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	assert.Contains(t, prompt, "Instructions")
	assert.Contains(t, prompt, "Implement")
	assert.Contains(t, prompt, "verification")
}

func TestBuilderBuildUserPrompt_NilTask(t *testing.T) {
	builder := NewBuilder(nil)
	ctx := IterationContext{Task: nil}

	_, err := builder.BuildUserPrompt(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task is required")
}

func TestBuilderBuildUserPrompt_SizeLimits(t *testing.T) {
	opts := &SizeOptions{
		MaxPromptBytes:   1000,
		MaxPatternsBytes: 50,
		MaxDiffBytes:     50,
		MaxFailureBytes:  50,
	}
	builder := NewBuilder(opts)

	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create large content that should be trimmed
	largePatterns := ""
	for i := 0; i < 100; i++ {
		largePatterns += "- This is a very long pattern line that should be truncated\n"
	}

	largeDiff := ""
	for i := 0; i < 100; i++ {
		largeDiff += "file" + string(rune('a'+i%26)) + ".go | 100 ++++++\n"
	}

	ctx := IterationContext{
		Task:             task,
		CodebasePatterns: largePatterns,
		DiffStat:         largeDiff,
	}

	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	// Patterns and diff should be truncated
	assert.Contains(t, prompt, "truncated")
}

func TestBuilderBuildUserPrompt_EmptyPatterns(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{
		Task:             task,
		CodebasePatterns: "",
	}

	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	// Should not include patterns section header when empty
	assert.NotContains(t, prompt, "Codebase Patterns")
}

func TestBuilderBuildUserPrompt_EmptyDiff(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{
		Task:     task,
		DiffStat: "",
	}

	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	// Should not include git diff section when empty
	assert.NotContains(t, prompt, "Git Status")
}

func TestBuilderBuildUserPrompt_ChangedFilesWithoutDiff(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{
		Task:         task,
		DiffStat:     "",
		ChangedFiles: []string{"file1.go", "file2.go"},
	}

	prompt, err := builder.BuildUserPrompt(ctx)

	require.NoError(t, err)
	// Changed files section should still appear even without diff stat
	assert.Contains(t, prompt, "file1.go")
	assert.Contains(t, prompt, "file2.go")
}

func TestTruncateWithMarker(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxBytes int
		want     string
	}{
		{
			name:     "no truncation needed",
			input:    "short string",
			maxBytes: 100,
			want:     "short string",
		},
		{
			name:     "truncation needed",
			input:    "this is a longer string that needs truncation",
			maxBytes: 20,
			want:     "this is a longer... [truncated]",
		},
		{
			name:     "zero max bytes disables truncation",
			input:    "any string",
			maxBytes: 0,
			want:     "any string",
		},
		{
			name:     "exact max bytes",
			input:    "exact",
			maxBytes: 5,
			want:     "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateWithMarker(tt.input, tt.maxBytes)
			if tt.maxBytes > 0 && len(tt.input) > tt.maxBytes {
				assert.Contains(t, got, "[truncated]")
				assert.LessOrEqual(t, len(got), tt.maxBytes+len("... [truncated]"))
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildResult(t *testing.T) {
	result := BuildResult{
		SystemPrompt: "system prompt content",
		UserPrompt:   "user prompt content",
	}

	assert.Equal(t, "system prompt content", result.SystemPrompt)
	assert.Equal(t, "user prompt content", result.UserPrompt)
}

func TestBuilderBuild(t *testing.T) {
	builder := NewBuilder(nil)
	task := &taskstore.Task{
		ID:          "test-task",
		Title:       "Test Task",
		Description: "Test description",
		Status:      taskstore.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := IterationContext{Task: task}
	result, err := builder.Build(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, result.SystemPrompt)
	assert.NotEmpty(t, result.UserPrompt)
}
