// Package git provides Git operations for the Ralph harness.
package git

import (
	"context"
	"errors"
	"fmt"
)

// Sentinel errors for common Git failures.
var (
	// ErrNotAGitRepo indicates the directory is not a git repository.
	ErrNotAGitRepo = errors.New("not a git repository")

	// ErrNoChanges indicates there are no changes to commit.
	ErrNoChanges = errors.New("no changes to commit")

	// ErrBranchExists indicates the branch already exists.
	ErrBranchExists = errors.New("branch already exists")

	// ErrCommitFailed indicates the commit operation failed.
	ErrCommitFailed = errors.New("commit failed")
)

// GitError represents a Git command error with additional context.
type GitError struct {
	// Command is the git command that failed.
	Command string
	// Output is the stderr/stdout output from the command.
	Output string
	// Err is the underlying error (typically a sentinel error).
	Err error
}

// Error returns a formatted error message.
func (e *GitError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("git command %q failed: %s", e.Command, e.Output)
	}
	return fmt.Sprintf("git command %q failed", e.Command)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *GitError) Unwrap() error {
	return e.Err
}

// Manager defines the interface for Git operations.
// It provides methods for branch management, commit operations, and diff tracking.
type Manager interface {
	// EnsureBranch ensures a branch exists and switches to it.
	// If the branch doesn't exist, it creates it.
	// If the branch already exists, it switches to it.
	EnsureBranch(ctx context.Context, branchName string) error

	// GetCurrentCommit returns the current HEAD commit hash.
	GetCurrentCommit(ctx context.Context) (string, error)

	// HasChanges returns true if there are uncommitted changes in the working tree.
	// This includes both staged and unstaged changes.
	HasChanges(ctx context.Context) (bool, error)

	// GetDiffStat returns the diff stat output for uncommitted changes.
	// This shows a summary of files changed with insertion/deletion counts.
	GetDiffStat(ctx context.Context) (string, error)

	// GetChangedFiles returns a list of files with uncommitted changes.
	// This includes both staged and unstaged files.
	GetChangedFiles(ctx context.Context) ([]string, error)

	// Commit creates a commit with the given message and returns the commit hash.
	// It stages all changes before committing.
	// Returns ErrNoChanges if there are no changes to commit.
	Commit(ctx context.Context, message string) (string, error)

	// GetCurrentBranch returns the name of the current branch.
	GetCurrentBranch(ctx context.Context) (string, error)
}
