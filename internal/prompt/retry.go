package prompt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yarlson/go-ralph/internal/taskstore"
)

// RetryContext contains context needed to build a retry prompt after verification failure.
type RetryContext struct {
	// Task is the task being retried.
	Task *taskstore.Task

	// FailureOutput is the trimmed verification failure output.
	FailureOutput string

	// FailureSignature is the hash signature of the failure for tracking.
	FailureSignature string

	// UserFeedback is any user-provided feedback for this retry.
	UserFeedback string

	// AttemptNumber is the retry attempt number (1-indexed).
	// 0 means not set.
	AttemptNumber int
}

// BuildRetrySystemPrompt builds the system prompt for retry iterations.
// It emphasizes fix-only approach and analyzing the failure.
func (b *Builder) BuildRetrySystemPrompt() string {
	return `You are a coding agent working within the Ralph harness. This is a RETRY after verification failure.

## Your Role
You must FIX the verification failure for this task. This is a retry - focus only on fixing the failure.

## Rules
1. ANALYZE the verification failure output carefully.
2. FIX ONLY what is necessary to make verification pass. Do not add new features.
3. Make MINIMAL, surgical changes. Do not refactor or improve unrelated code.
4. Run verification commands to check your fix. Do not declare completion until they pass.
5. Do NOT commit changes - the harness will commit after verification passes.
6. Update .ralph/progress.md with what you fixed and what you learned.

## Fix-Only Directive
This is a RETRY. You must:
- Focus on the specific verification failure
- Not add new functionality
- Not refactor code beyond what's needed for the fix
- Not make "improvements" that are unrelated to the failure

## Completion
When done:
- All verification commands must pass
- Stop and let the harness verify and commit
`
}

// BuildRetryPrompt builds the user prompt for a retry iteration.
func (b *Builder) BuildRetryPrompt(ctx RetryContext) (string, error) {
	if ctx.Task == nil {
		return "", errors.New("task is required")
	}

	var sb strings.Builder

	// Retry header with attempt number
	if ctx.AttemptNumber > 0 {
		_, _ = fmt.Fprintf(&sb, "## RETRY: %s (attempt %d)\n\n", ctx.Task.Title, ctx.AttemptNumber)
	} else {
		_, _ = fmt.Fprintf(&sb, "## RETRY: %s\n\n", ctx.Task.Title)
	}

	// Fix-only directive
	sb.WriteString("**This is a retry after verification failure. Fix only what is needed to pass verification.**\n\n")

	// Verification failure section
	if ctx.FailureOutput != "" {
		sb.WriteString("### Verification Failed\n\n")
		sb.WriteString("The following verification failure occurred:\n\n")
		sb.WriteString("```\n")
		failureOutput := truncateWithMarker(ctx.FailureOutput, b.opts.MaxFailureBytes)
		sb.WriteString(failureOutput)
		sb.WriteString("\n```\n\n")
	}

	// Failure signature (for debugging context)
	if ctx.FailureSignature != "" {
		_, _ = fmt.Fprintf(&sb, "Failure signature: `%s`\n\n", ctx.FailureSignature)
	}

	// User feedback
	if ctx.UserFeedback != "" {
		sb.WriteString("### User Feedback\n\n")
		sb.WriteString(ctx.UserFeedback)
		sb.WriteString("\n\n")
	}

	// Task description (for context)
	sb.WriteString("### Task Description\n\n")
	sb.WriteString(ctx.Task.Description)
	sb.WriteString("\n\n")

	// Acceptance criteria
	if len(ctx.Task.Acceptance) > 0 {
		sb.WriteString("### Acceptance Criteria\n")
		for _, a := range ctx.Task.Acceptance {
			_, _ = fmt.Fprintf(&sb, "- %s\n", a)
		}
		sb.WriteString("\n")
	}

	// Verification commands
	if len(ctx.Task.Verify) > 0 {
		sb.WriteString("### Verification Commands\n")
		sb.WriteString("Run these commands to verify your fix:\n")
		for _, v := range ctx.Task.Verify {
			_, _ = fmt.Fprintf(&sb, "- `%s`\n", strings.Join(v, " "))
		}
		sb.WriteString("\n")
	}

	// Instructions
	sb.WriteString("### Instructions\n\n")
	sb.WriteString("1. Analyze the verification failure above.\n")
	sb.WriteString("2. Identify the root cause of the failure.\n")
	sb.WriteString("3. Make the minimal fix necessary.\n")
	sb.WriteString("4. Run verification commands to confirm the fix.\n")
	sb.WriteString("5. Do not commit - the harness will commit after verification.\n")

	return sb.String(), nil
}

// BuildRetry builds both system and user prompts for a retry iteration.
func (b *Builder) BuildRetry(ctx RetryContext) (*BuildResult, error) {
	systemPrompt := b.BuildRetrySystemPrompt()

	userPrompt, err := b.BuildRetryPrompt(ctx)
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}, nil
}
