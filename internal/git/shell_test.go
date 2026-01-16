package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo with default branch name "main"
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to init git repo: %s", string(out))

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to config user.email: %s", string(out))

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to config user.name: %s", string(out))

	// Disable GPG signing for test commits
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to disable gpg signing: %s", string(out))

	return dir
}

// createTestFile creates a file in the given directory
func createTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	require.NoError(t, err, "failed to create test file")
}

// commitTestFile adds and commits a file
func commitTestFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	createTestFile(t, dir, name, content)

	cmd := exec.Command("git", "add", name)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to stage file: %s", string(out))

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to commit file: %s", string(out))
}

func TestShellManager_ImplementsInterface(t *testing.T) {
	// Verify ShellManager implements Manager interface
	var _ Manager = (*ShellManager)(nil)
}

func TestNewShellManager(t *testing.T) {
	dir := t.TempDir()
	mgr := NewShellManager(dir, "ralph/")

	assert.Equal(t, dir, mgr.workDir)
	assert.Equal(t, "ralph/", mgr.branchPrefix)
}

func TestShellManager_GetCurrentBranch(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	// Need at least one commit to have a branch
	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	branch, err := mgr.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	// Git default branch is typically "main" or "master"
	assert.True(t, branch == "main" || branch == "master", "expected main or master, got %s", branch)
}

func TestShellManager_GetCurrentBranch_NotGitRepo(t *testing.T) {
	dir := t.TempDir() // Not initialized as git repo
	mgr := NewShellManager(dir, "ralph/")

	_, err := mgr.GetCurrentBranch(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotAGitRepo))
}

func TestShellManager_GetCurrentCommit(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	commit, err := mgr.GetCurrentCommit(context.Background())
	require.NoError(t, err)
	assert.Len(t, commit, 40, "commit hash should be 40 characters")
}

func TestShellManager_GetCurrentCommit_NoCommits(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	// Empty repo with no commits
	_, err := mgr.GetCurrentCommit(context.Background())
	require.Error(t, err)
}

func TestShellManager_HasChanges_NoChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	has, err := mgr.HasChanges(context.Background())
	require.NoError(t, err)
	assert.False(t, has)
}

func TestShellManager_HasChanges_WithChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Modify the file
	createTestFile(t, dir, "README.md", "# Test Modified")

	has, err := mgr.HasChanges(context.Background())
	require.NoError(t, err)
	assert.True(t, has)
}

func TestShellManager_HasChanges_WithStagedChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Create and stage a new file
	createTestFile(t, dir, "new.txt", "new content")
	cmd := exec.Command("git", "add", "new.txt")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	has, err := mgr.HasChanges(context.Background())
	require.NoError(t, err)
	assert.True(t, has)
}

func TestShellManager_HasChanges_UntrackedFiles(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Create untracked file
	createTestFile(t, dir, "untracked.txt", "untracked content")

	has, err := mgr.HasChanges(context.Background())
	require.NoError(t, err)
	assert.True(t, has)
}

func TestShellManager_GetDiffStat(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Modify the file
	createTestFile(t, dir, "README.md", "# Test Modified\n\nSome content")

	stat, err := mgr.GetDiffStat(context.Background())
	require.NoError(t, err)
	assert.Contains(t, stat, "README.md")
	assert.Contains(t, stat, "1 file changed")
}

func TestShellManager_GetDiffStat_NoChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	stat, err := mgr.GetDiffStat(context.Background())
	require.NoError(t, err)
	assert.Empty(t, stat)
}

func TestShellManager_GetChangedFiles(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Modify existing and add new file
	createTestFile(t, dir, "README.md", "# Test Modified")
	createTestFile(t, dir, "new.txt", "new content")

	files, err := mgr.GetChangedFiles(context.Background())
	require.NoError(t, err)
	assert.Contains(t, files, "README.md")
	assert.Contains(t, files, "new.txt")
}

func TestShellManager_GetChangedFiles_NoChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	files, err := mgr.GetChangedFiles(context.Background())
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestShellManager_Commit(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Create changes to commit
	createTestFile(t, dir, "new.txt", "new content")

	hash, err := mgr.Commit(context.Background(), "feat: add new file")
	require.NoError(t, err)
	assert.Len(t, hash, 40, "commit hash should be 40 characters")

	// Verify the commit was made
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "feat: add new file")
}

func TestShellManager_Commit_NoChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// No changes to commit
	_, err := mgr.Commit(context.Background(), "empty commit")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoChanges))
}

func TestShellManager_Commit_StagedChanges(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Stage a new file
	createTestFile(t, dir, "staged.txt", "staged content")
	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	hash, err := mgr.Commit(context.Background(), "feat: add staged file")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestShellManager_EnsureBranch_CreateNew(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	err := mgr.EnsureBranch(context.Background(), "feature-test")
	require.NoError(t, err)

	branch, err := mgr.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ralph/feature-test", branch)
}

func TestShellManager_EnsureBranch_SwitchToExisting(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Create branch first
	cmd := exec.Command("git", "checkout", "-b", "ralph/existing-branch")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Switch back to main/master
	cmd = exec.Command("git", "checkout", "-")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Now EnsureBranch should switch to existing branch
	err := mgr.EnsureBranch(context.Background(), "existing-branch")
	require.NoError(t, err)

	branch, err := mgr.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ralph/existing-branch", branch)
}

func TestShellManager_EnsureBranch_AlreadyOnBranch(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	// Create and switch to branch
	err := mgr.EnsureBranch(context.Background(), "test-branch")
	require.NoError(t, err)

	// Calling again should be idempotent
	err = mgr.EnsureBranch(context.Background(), "test-branch")
	require.NoError(t, err)

	branch, err := mgr.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ralph/test-branch", branch)
}

func TestShellManager_ContextCancellation(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := mgr.GetCurrentBranch(ctx)
	require.Error(t, err)
}

func TestShellManager_ContextTimeout(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "ralph/")

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the context time to expire
	time.Sleep(10 * time.Millisecond)

	_, err := mgr.GetCurrentBranch(ctx)
	require.Error(t, err)
}

func TestShellManager_NotGitRepo(t *testing.T) {
	dir := t.TempDir() // Not initialized as git repo
	mgr := NewShellManager(dir, "ralph/")

	tests := []struct {
		name string
		fn   func() error
	}{
		{"GetCurrentBranch", func() error { _, err := mgr.GetCurrentBranch(context.Background()); return err }},
		{"GetCurrentCommit", func() error { _, err := mgr.GetCurrentCommit(context.Background()); return err }},
		{"HasChanges", func() error { _, err := mgr.HasChanges(context.Background()); return err }},
		{"GetDiffStat", func() error { _, err := mgr.GetDiffStat(context.Background()); return err }},
		{"GetChangedFiles", func() error { _, err := mgr.GetChangedFiles(context.Background()); return err }},
		{"EnsureBranch", func() error { return mgr.EnsureBranch(context.Background(), "test") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrNotAGitRepo), "expected ErrNotAGitRepo, got %v", err)
		})
	}
}

func TestShellManager_EmptyBranchPrefix(t *testing.T) {
	dir := setupTestRepo(t)
	mgr := NewShellManager(dir, "") // Empty prefix

	commitTestFile(t, dir, "README.md", "# Test", "initial commit")

	err := mgr.EnsureBranch(context.Background(), "feature-test")
	require.NoError(t, err)

	branch, err := mgr.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "feature-test", branch) // No prefix
}
