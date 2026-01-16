// Package prompt provides prompt packaging for Claude Code iterations.
package prompt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yarlson/go-ralph/internal/taskstore"
)

// IterationContext contains all the context needed to build an iteration prompt.
type IterationContext struct {
	// Task is the task to implement.
	Task *taskstore.Task

	// CodebasePatterns is the Codebase Patterns section from progress.md.
	CodebasePatterns string

	// DiffStat is the output of git diff --stat.
	DiffStat string

	// ChangedFiles is the list of files with uncommitted changes.
	ChangedFiles []string

	// FailureOutput is the trimmed verification failure output (for retries).
	FailureOutput string

	// UserFeedback is any user-provided feedback (for retries).
	UserFeedback string

	// IsRetry indicates if this is a retry of a failed task.
	IsRetry bool
}

// SizeOptions configures the maximum sizes for various prompt components.
type SizeOptions struct {
	// MaxPromptBytes is the maximum total prompt size in bytes.
	MaxPromptBytes int

	// MaxPatternsBytes is the maximum size of the patterns section.
	MaxPatternsBytes int

	// MaxDiffBytes is the maximum size of the diff/changed files section.
	MaxDiffBytes int

	// MaxFailureBytes is the maximum size of the failure output section.
	MaxFailureBytes int
}

// DefaultSizeOptions returns sensible default size options.
func DefaultSizeOptions() SizeOptions {
	return SizeOptions{
		MaxPromptBytes:   8000,
		MaxPatternsBytes: 2000,
		MaxDiffBytes:     1000,
		MaxFailureBytes:  2000,
	}
}

// Validate checks that all size options are non-negative.
func (o SizeOptions) Validate() error {
	if o.MaxPromptBytes < 0 {
		return errors.New("max prompt bytes cannot be negative")
	}
	if o.MaxPatternsBytes < 0 {
		return errors.New("max patterns bytes cannot be negative")
	}
	if o.MaxDiffBytes < 0 {
		return errors.New("max diff bytes cannot be negative")
	}
	if o.MaxFailureBytes < 0 {
		return errors.New("max failure bytes cannot be negative")
	}
	return nil
}

// BuildResult contains the built prompts ready for Claude invocation.
type BuildResult struct {
	// SystemPrompt is the system prompt with harness instructions.
	SystemPrompt string

	// UserPrompt is the user prompt with task details and context.
	UserPrompt string
}

// Builder builds iteration prompts for Claude Code.
type Builder struct {
	opts SizeOptions
}

// NewBuilder creates a new prompt builder with the given options.
// If opts is nil, default options are used.
func NewBuilder(opts *SizeOptions) *Builder {
	if opts == nil {
		defaultOpts := DefaultSizeOptions()
		opts = &defaultOpts
	}
	return &Builder{opts: *opts}
}

// BuildSystemPrompt builds the system prompt with harness instructions.
func (b *Builder) BuildSystemPrompt() string {
	return `You are a coding agent working within the Ralph harness.

## Your Role
You implement one task at a time. The harness manages task selection, verification, and commits.

## Rules
1. Implement ONLY the task described below. Do not work on other tasks.
2. Run verification commands to check your work. Fix any failures before declaring completion.
3. Do NOT commit changes - the harness will commit after verification passes.
4. Update .ralph/progress.md with: what changed, files touched, learnings/gotchas.
5. Update CLAUDE.md ONLY with durable guidance (no task-specific notes).
6. Prefer minimal, surgical changes. Avoid over-engineering.
7. Follow existing codebase patterns and conventions.

## Completion
When done:
- All verification commands must pass
- Stop and let the harness verify and commit
`
}

// BuildUserPrompt builds the user prompt with task details and context.
func (b *Builder) BuildUserPrompt(ctx IterationContext) (string, error) {
	if ctx.Task == nil {
		return "", errors.New("task is required")
	}

	var sb strings.Builder

	// Task header
	fmt.Fprintf(&sb, "## Task: %s\n\n", ctx.Task.Title)

	// Description
	fmt.Fprintf(&sb, "### Description\n%s\n\n", ctx.Task.Description)

	// Acceptance criteria
	if len(ctx.Task.Acceptance) > 0 {
		sb.WriteString("### Acceptance Criteria\n")
		for _, a := range ctx.Task.Acceptance {
			fmt.Fprintf(&sb, "- %s\n", a)
		}
		sb.WriteString("\n")
	}

	// Verification commands
	if len(ctx.Task.Verify) > 0 {
		sb.WriteString("### Verification Commands\n")
		sb.WriteString("Run these commands to verify your changes:\n")
		for _, v := range ctx.Task.Verify {
			fmt.Fprintf(&sb, "- `%s`\n", strings.Join(v, " "))
		}
		sb.WriteString("\n")
	}

	// Codebase patterns
	if ctx.CodebasePatterns != "" {
		patterns := truncateWithMarker(ctx.CodebasePatterns, b.opts.MaxPatternsBytes)
		sb.WriteString("### Codebase Patterns\n")
		sb.WriteString("Follow these patterns discovered during implementation:\n")
		sb.WriteString(patterns)
		sb.WriteString("\n\n")
	}

	// Git status / diff
	if ctx.DiffStat != "" || len(ctx.ChangedFiles) > 0 {
		sb.WriteString("### Git Status\n")
		if ctx.DiffStat != "" {
			diffStat := truncateWithMarker(ctx.DiffStat, b.opts.MaxDiffBytes)
			sb.WriteString("Current diff stat:\n```\n")
			sb.WriteString(diffStat)
			sb.WriteString("\n```\n")
		}
		if len(ctx.ChangedFiles) > 0 {
			sb.WriteString("Changed files:\n")
			for _, f := range ctx.ChangedFiles {
				fmt.Fprintf(&sb, "- `%s`\n", f)
			}
		}
		sb.WriteString("\n")
	}

	// Instructions
	sb.WriteString("### Instructions\n")
	sb.WriteString("1. Implement the task according to the description and acceptance criteria.\n")
	sb.WriteString("2. Run the verification commands and fix any failures.\n")
	sb.WriteString("3. Do not commit - the harness will commit after verification.\n")
	sb.WriteString("4. Update .ralph/progress.md with what changed and learnings.\n")

	return sb.String(), nil
}

// Build builds both system and user prompts from the given context.
func (b *Builder) Build(ctx IterationContext) (*BuildResult, error) {
	systemPrompt := b.BuildSystemPrompt()

	userPrompt, err := b.BuildUserPrompt(ctx)
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}, nil
}

// truncateWithMarker truncates a string to maxBytes and adds a marker if truncated.
// If maxBytes is 0, no truncation is performed.
func truncateWithMarker(s string, maxBytes int) string {
	if maxBytes == 0 || len(s) <= maxBytes {
		return s
	}

	marker := "... [truncated]"
	truncateAt := max(maxBytes, 0)
	return s[:truncateAt] + marker
}
