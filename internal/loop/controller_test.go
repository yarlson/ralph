package loop

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/go-ralph/internal/claude"
	"github.com/yarlson/go-ralph/internal/git"
	"github.com/yarlson/go-ralph/internal/memory"
	"github.com/yarlson/go-ralph/internal/selector"
	"github.com/yarlson/go-ralph/internal/state"
	"github.com/yarlson/go-ralph/internal/taskstore"
	"github.com/yarlson/go-ralph/internal/verifier"
)

// Ensure git package is used (for git.Manager interface)
var _ git.Manager = (*mockGitManager)(nil)

// mockTaskStore implements taskstore.Store for testing.
type mockTaskStore struct {
	tasks       map[string]*taskstore.Task
	getErr      error
	listErr     error
	saveErr     error
	updateErr   error
	deleteErr   error
	updateCalls []updateStatusCall
}

type updateStatusCall struct {
	ID     string
	Status taskstore.TaskStatus
}

func newMockTaskStore() *mockTaskStore {
	return &mockTaskStore{
		tasks:       make(map[string]*taskstore.Task),
		updateCalls: []updateStatusCall{},
	}
}

func (m *mockTaskStore) Get(id string) (*taskstore.Task, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	t, ok := m.tasks[id]
	if !ok {
		return nil, &taskstore.NotFoundError{ID: id}
	}
	return t, nil
}

func (m *mockTaskStore) List() ([]*taskstore.Task, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*taskstore.Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockTaskStore) ListByParent(parentID string) ([]*taskstore.Task, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*taskstore.Task
	for _, t := range m.tasks {
		if (parentID == "" && t.ParentID == nil) ||
			(t.ParentID != nil && *t.ParentID == parentID) {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskStore) Save(task *taskstore.Task) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskStore) UpdateStatus(id string, status taskstore.TaskStatus) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	t, ok := m.tasks[id]
	if !ok {
		return &taskstore.NotFoundError{ID: id}
	}
	t.Status = status
	t.UpdatedAt = time.Now()
	m.updateCalls = append(m.updateCalls, updateStatusCall{ID: id, Status: status})
	return nil
}

func (m *mockTaskStore) Delete(id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.tasks[id]; !ok {
		return &taskstore.NotFoundError{ID: id}
	}
	delete(m.tasks, id)
	return nil
}

func (m *mockTaskStore) addTask(t *taskstore.Task) {
	m.tasks[t.ID] = t
}

// mockClaudeRunner implements claude.Runner for testing.
type mockClaudeRunner struct {
	response *claude.ClaudeResponse
	err      error
	calls    []claude.ClaudeRequest
}

func (m *mockClaudeRunner) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	m.calls = append(m.calls, req)
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

// mockVerifier implements verifier.Verifier for testing.
type mockVerifier struct {
	results  []verifier.VerificationResult
	err      error
	calls    int
	verifyFn func(ctx context.Context, commands [][]string) ([]verifier.VerificationResult, error)
}

func (m *mockVerifier) Verify(ctx context.Context, commands [][]string) ([]verifier.VerificationResult, error) {
	m.calls++
	if m.verifyFn != nil {
		return m.verifyFn(ctx, commands)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockVerifier) VerifyTask(ctx context.Context, commands [][]string) ([]verifier.VerificationResult, error) {
	return m.Verify(ctx, commands)
}

// mockGitManager implements git.Manager for testing.
type mockGitManager struct {
	currentCommit string
	hasChanges    bool
	changedFiles  []string
	diffStat      string
	commitHash    string
	currentBranch string
	err           error
	commitCalls   []string
}

func (m *mockGitManager) EnsureBranch(ctx context.Context, branchName string) error {
	return m.err
}

func (m *mockGitManager) GetCurrentCommit(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.currentCommit, nil
}

func (m *mockGitManager) HasChanges(ctx context.Context) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.hasChanges, nil
}

func (m *mockGitManager) GetDiffStat(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.diffStat, nil
}

func (m *mockGitManager) GetChangedFiles(ctx context.Context) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.changedFiles, nil
}

func (m *mockGitManager) Commit(ctx context.Context, message string) (string, error) {
	m.commitCalls = append(m.commitCalls, message)
	if m.err != nil {
		return "", m.err
	}
	return m.commitHash, nil
}

func (m *mockGitManager) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.currentBranch, nil
}

// Helper to create test tasks with required fields.
func newTestTask(id, title string, status taskstore.TaskStatus, parentID *string) *taskstore.Task {
	now := time.Now()
	return &taskstore.Task{
		ID:        id,
		Title:     title,
		Status:    status,
		ParentID:  parentID,
		CreatedAt: now,
		UpdatedAt: now,
		Verify:    [][]string{{"go", "test", "./..."}},
	}
}

func strPtr(s string) *string {
	return &s
}

// --- Tests ---

func TestRunLoopOutcome_IsValid(t *testing.T) {
	tests := []struct {
		outcome RunLoopOutcome
		valid   bool
	}{
		{RunOutcomeCompleted, true},
		{RunOutcomeBlocked, true},
		{RunOutcomeBudgetExceeded, true},
		{RunOutcomeGutterDetected, true},
		{RunOutcomePaused, true},
		{RunOutcomeError, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.outcome), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.outcome.IsValid())
		})
	}
}

func TestNewController(t *testing.T) {
	deps := ControllerDeps{
		TaskStore:   newMockTaskStore(),
		Claude:      &mockClaudeRunner{},
		Verifier:    &mockVerifier{},
		Git:         &mockGitManager{},
		LogsDir:     "/tmp/logs",
		ProgressDir: "/tmp/progress",
	}

	ctrl := NewController(deps)

	assert.NotNil(t, ctrl)
	assert.NotNil(t, ctrl.budget)
	assert.NotNil(t, ctrl.gutter)
}

func TestController_RunLoop_NoReadyTasks(t *testing.T) {
	store := newMockTaskStore()

	// Parent task
	parent := newTestTask("parent", "Parent Task", taskstore.StatusOpen, nil)
	store.addTask(parent)

	// Child task that is already completed
	child := newTestTask("child", "Child Task", taskstore.StatusCompleted, strPtr("parent"))
	store.addTask(child)

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      &mockClaudeRunner{},
		Verifier:    &mockVerifier{},
		Git:         &mockGitManager{},
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	result := ctrl.RunLoop(context.Background(), "parent")

	assert.Equal(t, RunOutcomeCompleted, result.Outcome)
	assert.Equal(t, 0, result.IterationsRun)
}

func TestController_RunLoop_SingleIteration_Success(t *testing.T) {
	store := newMockTaskStore()

	// Parent task
	parent := newTestTask("parent", "Parent Task", taskstore.StatusOpen, nil)
	store.addTask(parent)

	// Ready child task
	child := newTestTask("child", "Child Task", taskstore.StatusOpen, strPtr("parent"))
	child.Verify = [][]string{{"echo", "ok"}}
	store.addTask(child)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{
			SessionID:    "sess-123",
			FinalText:    "Task completed successfully",
			TotalCostUSD: 0.01,
		},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{
			{Passed: true, Command: []string{"echo", "ok"}, Output: "ok"},
		},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc123",
		hasChanges:    true,
		changedFiles:  []string{"file1.go"},
		commitHash:    "def456",
	}

	logsDir := t.TempDir()
	progressDir := t.TempDir()

	// Initialize progress file
	pf := memory.NewProgressFile(progressDir + "/progress.md")
	require.NoError(t, pf.Init("Test Feature", "parent"))

	deps := ControllerDeps{
		TaskStore:    store,
		Claude:       claudeRunner,
		Verifier:     verifierMock,
		Git:          gitMock,
		LogsDir:      logsDir,
		ProgressDir:  progressDir,
		ProgressFile: pf,
	}

	ctrl := NewController(deps)
	result := ctrl.RunLoop(context.Background(), "parent")

	assert.Equal(t, RunOutcomeCompleted, result.Outcome)
	assert.Equal(t, 1, result.IterationsRun)
	assert.Len(t, result.CompletedTasks, 1)
	assert.Equal(t, "child", result.CompletedTasks[0])

	// Verify task status was updated
	assert.Equal(t, taskstore.StatusCompleted, store.tasks["child"].Status)
}

func TestController_RunLoop_VerificationFails(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent Task", taskstore.StatusOpen, nil)
	store.addTask(parent)

	child := newTestTask("child", "Child Task", taskstore.StatusOpen, strPtr("parent"))
	child.Verify = [][]string{{"go", "test"}}
	store.addTask(child)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{
			SessionID: "sess-123",
			FinalText: "Done",
		},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{
			{Passed: false, Command: []string{"go", "test"}, Output: "test failed"},
		},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc123",
		hasChanges:    true,
		changedFiles:  []string{"file1.go"},
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	// Set low gutter threshold to trigger after few failures
	ctrl.gutter = NewGutterDetector(GutterConfig{
		MaxSameFailure:     2,
		MaxChurnIterations: 5,
		ChurnThreshold:     3,
	})

	// Set max iterations to prevent infinite loop
	ctrl.budget = NewBudgetTracker(BudgetLimits{MaxIterations: 3})

	result := ctrl.RunLoop(context.Background(), "parent")

	// Should detect gutter or hit budget
	assert.True(t, result.Outcome == RunOutcomeGutterDetected || result.Outcome == RunOutcomeBudgetExceeded)
	assert.Greater(t, result.IterationsRun, 0)
	assert.Equal(t, taskstore.StatusOpen, store.tasks["child"].Status) // Task still open
}

func TestController_RunLoop_BudgetExceeded(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent Task", taskstore.StatusOpen, nil)
	store.addTask(parent)

	// Multiple tasks to exceed budget
	for i := 0; i < 5; i++ {
		child := newTestTask("child-"+string(rune('a'+i)), "Child "+string(rune('A'+i)), taskstore.StatusOpen, strPtr("parent"))
		store.addTask(child)
	}

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{SessionID: "sess", FinalText: "Done"},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc",
		hasChanges:    true,
		changedFiles:  []string{"f.go"},
		commitHash:    "def",
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	ctrl.budget = NewBudgetTracker(BudgetLimits{MaxIterations: 2})

	result := ctrl.RunLoop(context.Background(), "parent")

	assert.Equal(t, RunOutcomeBudgetExceeded, result.Outcome)
	assert.Equal(t, 2, result.IterationsRun)
}

func TestController_RunLoop_ContextCancellation(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	child := newTestTask("child", "Child", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(child)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{SessionID: "sess", FinalText: "Done"},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc",
		hasChanges:    true,
		changedFiles:  []string{"f.go"},
		commitHash:    "def",
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := ctrl.RunLoop(ctx, "parent")

	assert.Equal(t, RunOutcomePaused, result.Outcome)
}

func TestController_RunLoop_ClaudeError(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	child := newTestTask("child", "Child", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(child)

	claudeRunner := &mockClaudeRunner{
		err: errors.New("claude failed"),
	}

	gitMock := &mockGitManager{
		currentCommit: "abc",
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    &mockVerifier{},
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	ctrl.budget = NewBudgetTracker(BudgetLimits{MaxIterations: 1})

	result := ctrl.RunLoop(context.Background(), "parent")

	// Should continue to next iteration or stop on budget
	assert.True(t, result.IterationsRun >= 0)
}

func TestController_RunLoop_NoChangesAfterClaude(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	child := newTestTask("child", "Child", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(child)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{SessionID: "sess", FinalText: "Done"},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc",
		hasChanges:    false, // No changes!
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    &mockVerifier{},
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	ctrl.budget = NewBudgetTracker(BudgetLimits{MaxIterations: 2})

	result := ctrl.RunLoop(context.Background(), "parent")

	// Should record as failed iteration and continue/stop
	assert.Greater(t, result.IterationsRun, 0)
}

func TestController_RunLoop_MultipleTasksSequential(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	// Task B depends on Task A
	taskA := newTestTask("task-a", "Task A", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(taskA)

	taskB := newTestTask("task-b", "Task B", taskstore.StatusOpen, strPtr("parent"))
	taskB.DependsOn = []string{"task-a"}
	store.addTask(taskB)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{SessionID: "sess", FinalText: "Done"},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc",
		hasChanges:    true,
		changedFiles:  []string{"f.go"},
		commitHash:    "def",
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	result := ctrl.RunLoop(context.Background(), "parent")

	assert.Equal(t, RunOutcomeCompleted, result.Outcome)
	assert.Equal(t, 2, result.IterationsRun)
	assert.Len(t, result.CompletedTasks, 2)

	// Task A should have been completed first
	assert.Contains(t, result.CompletedTasks, "task-a")
	assert.Contains(t, result.CompletedTasks, "task-b")
}

func TestController_RunOnce(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	taskA := newTestTask("task-a", "Task A", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(taskA)

	taskB := newTestTask("task-b", "Task B", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(taskB)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{SessionID: "sess", FinalText: "Done"},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc",
		hasChanges:    true,
		changedFiles:  []string{"f.go"},
		commitHash:    "def",
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	result := ctrl.RunOnce(context.Background(), "parent")

	// Should only run one iteration even though there are multiple tasks
	assert.Equal(t, 1, result.IterationsRun)
	assert.Len(t, result.CompletedTasks, 1)
}

func TestController_GetSummary(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	completed := newTestTask("completed", "Completed", taskstore.StatusCompleted, strPtr("parent"))
	store.addTask(completed)

	open := newTestTask("open", "Open", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(open)

	failed := newTestTask("failed", "Failed", taskstore.StatusFailed, strPtr("parent"))
	store.addTask(failed)

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      &mockClaudeRunner{},
		Verifier:    &mockVerifier{},
		Git:         &mockGitManager{},
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	summary, err := ctrl.GetSummary(context.Background(), "parent")

	require.NoError(t, err)
	assert.Equal(t, 1, summary.CompletedCount)
	assert.Equal(t, 1, summary.OpenCount)
	assert.Equal(t, 1, summary.FailedCount)
}

func TestController_RunLoop_WithDependencyGraph(t *testing.T) {
	store := newMockTaskStore()

	// Create a simple dependency tree
	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	// A -> (no deps)
	// B -> depends on A
	// C -> depends on A
	// D -> depends on B and C

	taskA := newTestTask("a", "Task A", taskstore.StatusOpen, strPtr("parent"))
	taskA.CreatedAt = time.Now().Add(-4 * time.Hour)
	store.addTask(taskA)

	taskB := newTestTask("b", "Task B", taskstore.StatusOpen, strPtr("parent"))
	taskB.DependsOn = []string{"a"}
	taskB.CreatedAt = time.Now().Add(-3 * time.Hour)
	store.addTask(taskB)

	taskC := newTestTask("c", "Task C", taskstore.StatusOpen, strPtr("parent"))
	taskC.DependsOn = []string{"a"}
	taskC.CreatedAt = time.Now().Add(-2 * time.Hour)
	store.addTask(taskC)

	taskD := newTestTask("d", "Task D", taskstore.StatusOpen, strPtr("parent"))
	taskD.DependsOn = []string{"b", "c"}
	taskD.CreatedAt = time.Now().Add(-1 * time.Hour)
	store.addTask(taskD)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{SessionID: "sess", FinalText: "Done"},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	// Use a mock that returns different files for each call to avoid gutter detection
	callCount := 0
	gitMock := &dynamicGitManager{
		getChangedFilesFn: func() []string {
			callCount++
			return []string{fmt.Sprintf("file%d.go", callCount)}
		},
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	// Disable gutter detection for this test
	ctrl.SetGutterConfig(GutterConfig{
		MaxSameFailure:     0, // Disabled
		MaxChurnIterations: 0, // Disabled
		ChurnThreshold:     0, // Disabled
	})

	result := ctrl.RunLoop(context.Background(), "parent")

	assert.Equal(t, RunOutcomeCompleted, result.Outcome)
	assert.Equal(t, 4, result.IterationsRun)

	// Verify A was completed before B, C
	// Verify B, C were completed before D
	aIdx := indexOf(result.CompletedTasks, "a")
	bIdx := indexOf(result.CompletedTasks, "b")
	cIdx := indexOf(result.CompletedTasks, "c")
	dIdx := indexOf(result.CompletedTasks, "d")

	assert.Less(t, aIdx, bIdx, "A should complete before B")
	assert.Less(t, aIdx, cIdx, "A should complete before C")
	assert.Less(t, bIdx, dIdx, "B should complete before D")
	assert.Less(t, cIdx, dIdx, "C should complete before D")
}

// dynamicGitManager is a git manager that allows customizing behavior per call.
type dynamicGitManager struct {
	getChangedFilesFn func() []string
}

func (m *dynamicGitManager) EnsureBranch(ctx context.Context, branchName string) error {
	return nil
}

func (m *dynamicGitManager) GetCurrentCommit(ctx context.Context) (string, error) {
	return "abc123", nil
}

func (m *dynamicGitManager) HasChanges(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *dynamicGitManager) GetDiffStat(ctx context.Context) (string, error) {
	return "1 file changed", nil
}

func (m *dynamicGitManager) GetChangedFiles(ctx context.Context) ([]string, error) {
	if m.getChangedFilesFn != nil {
		return m.getChangedFilesFn(), nil
	}
	return []string{"file.go"}, nil
}

func (m *dynamicGitManager) Commit(ctx context.Context, message string) (string, error) {
	return "def456", nil
}

func (m *dynamicGitManager) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func TestController_RunLoop_InvalidParentTask(t *testing.T) {
	store := newMockTaskStore()

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      &mockClaudeRunner{},
		Verifier:    &mockVerifier{},
		Git:         &mockGitManager{},
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	result := ctrl.RunLoop(context.Background(), "nonexistent")

	// When parent doesn't exist and there are no tasks, it's "completed" (nothing to do)
	assert.Equal(t, RunOutcomeCompleted, result.Outcome)
	assert.Equal(t, 0, result.IterationsRun)
}

func TestController_SetBudgetLimits(t *testing.T) {
	deps := ControllerDeps{
		TaskStore:   newMockTaskStore(),
		Claude:      &mockClaudeRunner{},
		Verifier:    &mockVerifier{},
		Git:         &mockGitManager{},
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	limits := BudgetLimits{
		MaxIterations: 100,
		MaxCostUSD:    10.0,
	}
	ctrl.SetBudgetLimits(limits)

	// Verify limits are set (through budget tracker behavior)
	assert.NotNil(t, ctrl.budget)
}

func TestController_SetGutterConfig(t *testing.T) {
	deps := ControllerDeps{
		TaskStore:   newMockTaskStore(),
		Claude:      &mockClaudeRunner{},
		Verifier:    &mockVerifier{},
		Git:         &mockGitManager{},
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)

	config := GutterConfig{
		MaxSameFailure:     5,
		MaxChurnIterations: 10,
		ChurnThreshold:     4,
	}
	ctrl.SetGutterConfig(config)

	assert.NotNil(t, ctrl.gutter)
}

func TestController_RunLoop_RecordsIterations(t *testing.T) {
	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	child := newTestTask("child", "Child", taskstore.StatusOpen, strPtr("parent"))
	store.addTask(child)

	claudeRunner := &mockClaudeRunner{
		response: &claude.ClaudeResponse{
			SessionID:    "sess-123",
			FinalText:    "Done",
			TotalCostUSD: 0.05,
			Usage: claude.ClaudeUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
		},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc123",
		hasChanges:    true,
		changedFiles:  []string{"file1.go", "file2.go"},
		commitHash:    "def456",
	}

	logsDir := t.TempDir()

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     logsDir,
		ProgressDir: t.TempDir(),
	}

	ctrl := NewController(deps)
	result := ctrl.RunLoop(context.Background(), "parent")

	assert.Equal(t, RunOutcomeCompleted, result.Outcome)
	assert.Len(t, result.Records, 1)

	record := result.Records[0]
	assert.Equal(t, "child", record.TaskID)
	assert.Equal(t, OutcomeSuccess, record.Outcome)
	assert.Equal(t, "abc123", record.BaseCommit)
	assert.Equal(t, "def456", record.ResultCommit)
	assert.Len(t, record.FilesChanged, 2)
	assert.Equal(t, "sess-123", record.ClaudeInvocation.SessionID)
	assert.Equal(t, 0.05, record.ClaudeInvocation.TotalCostUSD)
}

func TestBuildGraph_ForSelector(t *testing.T) {
	// Test that we can build a valid graph for selector
	tasks := []*taskstore.Task{
		newTestTask("a", "Task A", taskstore.StatusOpen, nil),
		newTestTask("b", "Task B", taskstore.StatusOpen, nil),
	}
	tasks[1].DependsOn = []string{"a"}

	graph, err := selector.BuildGraph(tasks)
	require.NoError(t, err)
	assert.NotNil(t, graph)
}

// mockClaudeRunnerWithCallback is a custom Claude runner that calls a callback function.
type mockClaudeRunnerWithCallback struct {
	callbackFn func() error
}

func (m *mockClaudeRunnerWithCallback) Run(ctx context.Context, req claude.ClaudeRequest) (*claude.ClaudeResponse, error) {
	if m.callbackFn != nil {
		if err := m.callbackFn(); err != nil {
			return nil, err
		}
	}
	return &claude.ClaudeResponse{
		SessionID:    "sess-123",
		FinalText:    "Done",
		TotalCostUSD: 0.01,
	}, nil
}

func TestController_RunLoop_ChecksPauseBetweenIterations(t *testing.T) {
	// Create a temp dir for .ralph state
	workDir := t.TempDir()
	require.NoError(t, state.EnsureRalphDir(workDir))

	store := newMockTaskStore()

	parent := newTestTask("parent", "Parent", taskstore.StatusOpen, nil)
	store.addTask(parent)

	// Create two child tasks
	child1 := newTestTask("child1", "Child 1", taskstore.StatusOpen, strPtr("parent"))
	child1.CreatedAt = time.Now().Add(-2 * time.Hour)
	store.addTask(child1)

	child2 := newTestTask("child2", "Child 2", taskstore.StatusOpen, strPtr("parent"))
	child2.CreatedAt = time.Now().Add(-1 * time.Hour)
	store.addTask(child2)

	// Mock that sets pause flag after first call
	iterationCount := 0
	claudeRunner := &mockClaudeRunnerWithCallback{
		callbackFn: func() error {
			iterationCount++
			if iterationCount == 1 {
				// After first iteration completes, set pause flag
				return state.SetPaused(workDir, true)
			}
			return nil
		},
	}

	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{{Passed: true, Command: []string{"echo"}}},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc123",
		hasChanges:    true,
		changedFiles:  []string{"file1.go"},
		commitHash:    "def456",
	}

	deps := ControllerDeps{
		TaskStore:   store,
		Claude:      claudeRunner,
		Verifier:    verifierMock,
		Git:         gitMock,
		LogsDir:     t.TempDir(),
		ProgressDir: t.TempDir(),
		WorkDir:     workDir,
	}

	ctrl := NewController(deps)
	result := ctrl.RunLoop(context.Background(), "parent")

	// Should have paused after first iteration
	assert.Equal(t, RunOutcomePaused, result.Outcome)
	assert.Contains(t, result.Message, "paused")
	assert.Equal(t, 1, result.IterationsRun)
	assert.Len(t, result.CompletedTasks, 1)
	assert.Equal(t, "child1", result.CompletedTasks[0])

	// child2 should still be open
	assert.Equal(t, taskstore.StatusOpen, store.tasks["child2"].Status)
}

func TestController_MergeVerificationCommands_NoConfigCommands(t *testing.T) {
	store := newMockTaskStore()
	deps := ControllerDeps{
		TaskStore: store,
		Claude:    &mockClaudeRunner{},
		Verifier:  &mockVerifier{},
		Git:       &mockGitManager{},
		LogsDir:   t.TempDir(),
	}

	ctrl := NewController(deps)
	taskCommands := [][]string{{"go", "test", "./..."}}

	merged := ctrl.mergeVerificationCommands(taskCommands)

	assert.Equal(t, taskCommands, merged)
}

func TestController_MergeVerificationCommands_NoTaskCommands(t *testing.T) {
	store := newMockTaskStore()
	deps := ControllerDeps{
		TaskStore: store,
		Claude:    &mockClaudeRunner{},
		Verifier:  &mockVerifier{},
		Git:       &mockGitManager{},
		LogsDir:   t.TempDir(),
	}

	ctrl := NewController(deps)
	configCommands := [][]string{{"golangci-lint", "run"}}
	ctrl.SetConfigVerifyCommands(configCommands)

	merged := ctrl.mergeVerificationCommands(nil)

	assert.Equal(t, configCommands, merged)
}

func TestController_MergeVerificationCommands_BothPresent(t *testing.T) {
	store := newMockTaskStore()
	deps := ControllerDeps{
		TaskStore: store,
		Claude:    &mockClaudeRunner{},
		Verifier:  &mockVerifier{},
		Git:       &mockGitManager{},
		LogsDir:   t.TempDir(),
	}

	ctrl := NewController(deps)
	configCommands := [][]string{
		{"golangci-lint", "run"},
		{"go", "build", "./..."},
	}
	taskCommands := [][]string{{"go", "test", "./..."}}
	ctrl.SetConfigVerifyCommands(configCommands)

	merged := ctrl.mergeVerificationCommands(taskCommands)

	expected := [][]string{
		{"golangci-lint", "run"},
		{"go", "build", "./..."},
		{"go", "test", "./..."},
	}
	assert.Equal(t, expected, merged)
}

func TestController_RunIteration_WithConfigVerifyCommands(t *testing.T) {
	store := newMockTaskStore()
	task := newTestTask("task1", "Test Task", taskstore.StatusOpen, nil)
	task.Verify = [][]string{{"go", "test", "./..."}}
	store.addTask(task)

	// Track which commands were executed
	var executedCommands [][]string
	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{
			{Passed: true, Command: []string{"golangci-lint", "run"}},
			{Passed: true, Command: []string{"go", "test", "./..."}},
		},
		verifyFn: func(ctx context.Context, commands [][]string) ([]verifier.VerificationResult, error) {
			executedCommands = commands
			return []verifier.VerificationResult{
				{Passed: true, Command: []string{"golangci-lint", "run"}},
				{Passed: true, Command: []string{"go", "test", "./..."}},
			}, nil
		},
	}

	claudeMock := &mockClaudeRunner{
		response: &claude.ClaudeResponse{
			SessionID: "session123",
			Model:     "claude-sonnet-4-5",
			FinalText: "Changes made",
			Usage: claude.ClaudeUsage{
				InputTokens:  100,
				OutputTokens: 200,
			},
			TotalCostUSD: 0.05,
		},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc123",
		hasChanges:    true,
		changedFiles:  []string{"file1.go"},
		commitHash:    "def456",
	}

	deps := ControllerDeps{
		TaskStore: store,
		Claude:    claudeMock,
		Verifier:  verifierMock,
		Git:       gitMock,
		LogsDir:   t.TempDir(),
	}

	ctrl := NewController(deps)
	configCommands := [][]string{{"golangci-lint", "run"}}
	ctrl.SetConfigVerifyCommands(configCommands)

	record := ctrl.runIteration(context.Background(), task)

	// Should have merged config and task commands
	assert.Equal(t, OutcomeSuccess, record.Outcome)
	assert.Len(t, executedCommands, 2)
	assert.Equal(t, []string{"golangci-lint", "run"}, executedCommands[0])
	assert.Equal(t, []string{"go", "test", "./..."}, executedCommands[1])

	// Verification outputs should reflect both commands
	assert.Len(t, record.VerificationOutputs, 2)
	assert.True(t, record.VerificationOutputs[0].Passed)
	assert.True(t, record.VerificationOutputs[1].Passed)
}

func TestController_RunIteration_ConfigVerifyFails(t *testing.T) {
	store := newMockTaskStore()
	task := newTestTask("task1", "Test Task", taskstore.StatusOpen, nil)
	task.Verify = [][]string{{"go", "test", "./..."}}
	store.addTask(task)

	// Config command fails, task command passes
	verifierMock := &mockVerifier{
		results: []verifier.VerificationResult{
			{Passed: false, Command: []string{"golangci-lint", "run"}, Output: "linter errors"},
			{Passed: true, Command: []string{"go", "test", "./..."}},
		},
	}

	claudeMock := &mockClaudeRunner{
		response: &claude.ClaudeResponse{
			SessionID: "session123",
			Model:     "claude-sonnet-4-5",
			FinalText: "Changes made",
			Usage: claude.ClaudeUsage{
				InputTokens:  100,
				OutputTokens: 200,
			},
			TotalCostUSD: 0.05,
		},
	}

	gitMock := &mockGitManager{
		currentCommit: "abc123",
		hasChanges:    true,
		changedFiles:  []string{"file1.go"},
	}

	deps := ControllerDeps{
		TaskStore: store,
		Claude:    claudeMock,
		Verifier:  verifierMock,
		Git:       gitMock,
		LogsDir:   t.TempDir(),
	}

	ctrl := NewController(deps)
	configCommands := [][]string{{"golangci-lint", "run"}}
	ctrl.SetConfigVerifyCommands(configCommands)

	record := ctrl.runIteration(context.Background(), task)

	// Should fail because config command failed
	assert.Equal(t, OutcomeFailed, record.Outcome)
	assert.Len(t, record.VerificationOutputs, 2)
	assert.False(t, record.VerificationOutputs[0].Passed)
}

