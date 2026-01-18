package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// ShellManager implements the Manager interface by shelling out to git.
type ShellManager struct {
	workDir      string
	branchPrefix string
}

// NewShellManager creates a new ShellManager with the given working directory
// and branch prefix. The branch prefix is prepended to branch names when creating
// or switching branches (e.g., "ralph/" creates branches like "ralph/feature-name").
func NewShellManager(workDir, branchPrefix string) *ShellManager {
	return &ShellManager{
		workDir:      workDir,
		branchPrefix: branchPrefix,
	}
}

// runGit executes a git command and returns the combined output.
func (m *ShellManager) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		stderrLower := strings.ToLower(stderrStr)

		// Check if this is a "not a git repository" error
		if strings.Contains(stderrLower, "not a git repository") {
			return "", &GitError{
				Command: "git " + strings.Join(args, " "),
				Output:  stderrStr,
				Err:     ErrNotAGitRepo,
			}
		}

		// Check if this is an empty repo (no commits) error
		if strings.Contains(stderrLower, "ambiguous argument 'head'") ||
			strings.Contains(stderrLower, "unknown revision") {
			return "", &GitError{
				Command: "git " + strings.Join(args, " "),
				Output:  stderrStr,
				Err:     ErrNoCommits,
			}
		}

		return "", &GitError{
			Command: "git " + strings.Join(args, " "),
			Output:  stderrStr,
			Err:     err,
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Init initializes a new git repository in the working directory.
// Returns nil if already a git repository.
func (m *ShellManager) Init(ctx context.Context) error {
	// Check if already a git repository
	_, err := m.runGit(ctx, "rev-parse", "--git-dir")
	if err == nil {
		return nil // Already initialized
	}
	if !errors.Is(err, ErrNotAGitRepo) {
		return err // Unexpected error
	}

	// Initialize new repo
	_, err = m.runGit(ctx, "init")
	return err
}

// GetCurrentBranch returns the name of the current branch.
func (m *ShellManager) GetCurrentBranch(ctx context.Context) (string, error) {
	return m.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// getCurrentBranchSymbolic returns the current branch using symbolic-ref.
// This works even in empty repos with no commits.
func (m *ShellManager) getCurrentBranchSymbolic(ctx context.Context) (string, error) {
	return m.runGit(ctx, "symbolic-ref", "--short", "HEAD")
}

// GetCurrentCommit returns the current HEAD commit hash.
func (m *ShellManager) GetCurrentCommit(ctx context.Context) (string, error) {
	return m.runGit(ctx, "rev-parse", "HEAD")
}

// HasChanges returns true if there are uncommitted changes in the working tree.
// This includes staged changes, unstaged changes, and untracked files.
func (m *ShellManager) HasChanges(ctx context.Context) (bool, error) {
	// Check for staged or unstaged changes
	output, err := m.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return output != "", nil
}

// GetDiffStat returns the diff stat output for uncommitted changes.
func (m *ShellManager) GetDiffStat(ctx context.Context) (string, error) {
	return m.runGit(ctx, "diff", "--stat")
}

// GetChangedFiles returns a list of files with uncommitted changes.
// This includes staged, unstaged, and untracked files.
func (m *ShellManager) GetChangedFiles(ctx context.Context) ([]string, error) {
	output, err := m.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}

	var files []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if len(line) > 3 {
			// Format is "XY filename" where XY is status
			// Remove the status prefix (first 3 characters)
			file := strings.TrimSpace(line[2:])
			// Handle renamed files (format: "old -> new")
			if idx := strings.Index(file, " -> "); idx != -1 {
				file = file[idx+4:]
			}
			files = append(files, file)
		}
	}

	return files, nil
}

// Commit creates a commit with the given message and returns the commit hash.
// It stages all changes before committing.
func (m *ShellManager) Commit(ctx context.Context, message string) (string, error) {
	// Check if there are changes to commit
	hasChanges, err := m.HasChanges(ctx)
	if err != nil {
		return "", err
	}
	if !hasChanges {
		return "", &GitError{
			Command: "git commit",
			Output:  "nothing to commit, working tree clean",
			Err:     ErrNoChanges,
		}
	}

	// Stage all changes
	_, err = m.runGit(ctx, "add", "-A")
	if err != nil {
		return "", err
	}

	// Create commit
	_, err = m.runGit(ctx, "commit", "-m", message)
	if err != nil {
		return "", &GitError{
			Command: "git commit",
			Output:  err.Error(),
			Err:     ErrCommitFailed,
		}
	}

	// Get the commit hash
	return m.GetCurrentCommit(ctx)
}

// EnsureBranch ensures a branch exists and switches to it.
// The branch name is prefixed with the configured branch prefix.
// If the branch doesn't exist, it creates it. If it already exists, it switches to it.
// Handles empty repos (no commits) gracefully.
func (m *ShellManager) EnsureBranch(ctx context.Context, branchName string) error {
	fullBranchName := m.branchPrefix + branchName

	// Check if we're already on this branch
	currentBranch, err := m.GetCurrentBranch(ctx)
	if err != nil {
		// If repo has no commits, fall back to symbolic-ref which works in empty repos
		if errors.Is(err, ErrNoCommits) {
			currentBranch, err = m.getCurrentBranchSymbolic(ctx)
			if err != nil {
				return err
			}
			// In an empty repo, if we're on the target branch, we're done
			if currentBranch == fullBranchName {
				return nil
			}
			// In an empty repo, create the new branch (this will be orphan)
			_, err = m.runGit(ctx, "checkout", "-b", fullBranchName)
			return err
		}
		return err
	}
	if currentBranch == fullBranchName {
		return nil // Already on the branch
	}

	// Check if branch exists
	_, err = m.runGit(ctx, "rev-parse", "--verify", fullBranchName)
	if err == nil {
		// Branch exists, switch to it
		_, err = m.runGit(ctx, "checkout", fullBranchName)
		return err
	}

	// Branch doesn't exist, create and switch to it
	_, err = m.runGit(ctx, "checkout", "-b", fullBranchName)
	return err
}

// GetCommitMessage returns the commit message for the given commit hash.
// It uses the %B format to get the full commit message body.
func (m *ShellManager) GetCommitMessage(ctx context.Context, hash string) (string, error) {
	return m.runGit(ctx, "log", "-1", "--format=%B", hash)
}
