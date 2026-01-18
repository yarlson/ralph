package git

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockManager is a mock implementation of Manager for testing
type mockManager struct {
	currentBranch string
	currentCommit string
	hasChanges    bool
	diffStat      string
	changedFiles  []string
	commitHash    string
	commitMessage string
	err           error
}

func (m *mockManager) Init(_ context.Context) error {
	return m.err
}

func (m *mockManager) EnsureBranch(_ context.Context, _ string) error {
	return m.err
}

func (m *mockManager) GetCurrentCommit(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.currentCommit, nil
}

func (m *mockManager) HasChanges(_ context.Context) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.hasChanges, nil
}

func (m *mockManager) GetDiffStat(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.diffStat, nil
}

func (m *mockManager) GetChangedFiles(_ context.Context) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.changedFiles, nil
}

func (m *mockManager) Commit(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.commitHash, nil
}

func (m *mockManager) GetCurrentBranch(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.currentBranch, nil
}

func (m *mockManager) GetCommitMessage(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.commitMessage, nil
}

func TestManagerInterface(t *testing.T) {
	// Verify mockManager implements Manager interface
	var _ Manager = (*mockManager)(nil)
}

func TestMockManager_EnsureBranch(t *testing.T) {
	m := &mockManager{}
	err := m.EnsureBranch(context.Background(), "feature/test")
	assert.NoError(t, err)

	m.err = errors.New("branch error")
	err = m.EnsureBranch(context.Background(), "feature/test")
	assert.Error(t, err)
}

func TestMockManager_GetCurrentCommit(t *testing.T) {
	m := &mockManager{currentCommit: "abc123def456"}
	commit, err := m.GetCurrentCommit(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", commit)

	m.err = errors.New("commit error")
	_, err = m.GetCurrentCommit(context.Background())
	assert.Error(t, err)
}

func TestMockManager_HasChanges(t *testing.T) {
	m := &mockManager{hasChanges: true}
	has, err := m.HasChanges(context.Background())
	require.NoError(t, err)
	assert.True(t, has)

	m = &mockManager{hasChanges: false}
	has, err = m.HasChanges(context.Background())
	require.NoError(t, err)
	assert.False(t, has)

	m.err = errors.New("changes error")
	_, err = m.HasChanges(context.Background())
	assert.Error(t, err)
}

func TestMockManager_GetDiffStat(t *testing.T) {
	m := &mockManager{diffStat: " internal/git/manager.go | 50 +++++++\n 1 file changed, 50 insertions(+)"}
	stat, err := m.GetDiffStat(context.Background())
	require.NoError(t, err)
	assert.Contains(t, stat, "manager.go")

	m.err = errors.New("diff error")
	_, err = m.GetDiffStat(context.Background())
	assert.Error(t, err)
}

func TestMockManager_GetChangedFiles(t *testing.T) {
	files := []string{"internal/git/manager.go", "internal/git/manager_test.go"}
	m := &mockManager{changedFiles: files}
	result, err := m.GetChangedFiles(context.Background())
	require.NoError(t, err)
	assert.Equal(t, files, result)

	m.err = errors.New("files error")
	_, err = m.GetChangedFiles(context.Background())
	assert.Error(t, err)
}

func TestMockManager_Commit(t *testing.T) {
	m := &mockManager{commitHash: "abc123def456789"}
	hash, err := m.Commit(context.Background(), "feat: add feature")
	require.NoError(t, err)
	assert.Equal(t, "abc123def456789", hash)

	m.err = errors.New("commit error")
	_, err = m.Commit(context.Background(), "feat: add feature")
	assert.Error(t, err)
}

func TestMockManager_GetCurrentBranch(t *testing.T) {
	m := &mockManager{currentBranch: "main"}
	branch, err := m.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", branch)

	m.err = errors.New("branch error")
	_, err = m.GetCurrentBranch(context.Background())
	assert.Error(t, err)
}

func TestErrNotAGitRepo(t *testing.T) {
	err := ErrNotAGitRepo
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "git")
}

func TestErrNoChanges(t *testing.T) {
	err := ErrNoChanges
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no changes")
}

func TestErrBranchExists(t *testing.T) {
	err := ErrBranchExists
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "branch")
}

func TestErrCommitFailed(t *testing.T) {
	err := ErrCommitFailed
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "commit")
}

func TestGitError(t *testing.T) {
	gitErr := &GitError{
		Command: "git status",
		Output:  "fatal: not a git repository",
		Err:     ErrNotAGitRepo,
	}

	assert.Contains(t, gitErr.Error(), "git status")
	assert.Contains(t, gitErr.Error(), "not a git repository")
	assert.True(t, errors.Is(gitErr, ErrNotAGitRepo))
	assert.Equal(t, ErrNotAGitRepo, errors.Unwrap(gitErr))
}

func TestGitErrorWithNilErr(t *testing.T) {
	gitErr := &GitError{
		Command: "git status",
		Output:  "fatal: not a git repository",
		Err:     nil,
	}

	assert.Contains(t, gitErr.Error(), "git status")
	assert.Nil(t, errors.Unwrap(gitErr))
}
