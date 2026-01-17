package internal

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInteractive_WithFile(t *testing.T) {
	// Create a temp file (not a TTY)
	f, err := os.CreateTemp("", "test-interactive-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()

	// A regular file should not be interactive
	result := IsInteractive(f.Fd())
	assert.False(t, result, "a regular file should not be interactive")
}

func TestIsInteractive_WithInvalidFd(t *testing.T) {
	// An invalid file descriptor should return false
	result := IsInteractive(^uintptr(0)) // Invalid fd
	assert.False(t, result, "an invalid fd should not be interactive")
}

func TestIsInteractive_WithDevNull(t *testing.T) {
	// /dev/null is not a terminal
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip("could not open /dev/null")
	}
	defer func() { _ = f.Close() }()

	result := IsInteractive(f.Fd())
	assert.False(t, result, "/dev/null should not be interactive")
}

func TestConfirmUndo_ShowsAllInfo(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("no\n"))

	info := UndoConfirmationInfo{
		IterationID:           "abc123",
		CommitToResetTo:       "a1b2c3d4e5f6g7h8",
		TaskToReopen:          "task-42",
		FilesToRevert:         []string{"file1.go", "file2.go"},
		HasUncommittedChanges: false,
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.False(t, result)

	output := out.String()
	// Verify all information is shown
	assert.Contains(t, output, "abc123")              // Iteration ID
	assert.Contains(t, output, "a1b2c3d")             // Short commit hash
	assert.Contains(t, output, "Commit to reset to:") // Label
	assert.Contains(t, output, "Task to reopen:")     // Label
	assert.Contains(t, output, "task-42")             // Task ID
	assert.Contains(t, output, "Files to revert:")    // Label
	assert.Contains(t, output, "file1.go")            // File
	assert.Contains(t, output, "file2.go")            // File
}

func TestConfirmUndo_ShowsWarningForUncommittedChanges(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("no\n"))

	info := UndoConfirmationInfo{
		IterationID:           "abc123",
		CommitToResetTo:       "a1b2c3d4e5f6g7h8",
		HasUncommittedChanges: true,
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.False(t, result)

	output := out.String()
	assert.Contains(t, output, "WARNING")
	assert.Contains(t, output, "uncommitted changes")
}

func TestConfirmUndo_AcceptsYes(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("yes\n"))

	info := UndoConfirmationInfo{
		IterationID:     "abc123",
		CommitToResetTo: "a1b2c3d4e5f6g7h8",
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestConfirmUndo_AcceptsY(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("y\n"))

	info := UndoConfirmationInfo{
		IterationID:     "abc123",
		CommitToResetTo: "a1b2c3d4e5f6g7h8",
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestConfirmUndo_RejectsNo(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("no\n"))

	info := UndoConfirmationInfo{
		IterationID:     "abc123",
		CommitToResetTo: "a1b2c3d4e5f6g7h8",
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestConfirmUndo_RejectsOther(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("maybe\n"))

	info := UndoConfirmationInfo{
		IterationID:     "abc123",
		CommitToResetTo: "a1b2c3d4e5f6g7h8",
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestConfirmUndo_HidesTaskWhenEmpty(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("no\n"))

	info := UndoConfirmationInfo{
		IterationID:     "abc123",
		CommitToResetTo: "a1b2c3d4e5f6g7h8",
		TaskToReopen:    "", // Empty task
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.False(t, result)

	output := out.String()
	assert.NotContains(t, output, "Task to reopen:")
}

func TestConfirmUndo_HidesFilesWhenEmpty(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("no\n"))

	info := UndoConfirmationInfo{
		IterationID:     "abc123",
		CommitToResetTo: "a1b2c3d4e5f6g7h8",
		FilesToRevert:   nil, // Empty files
	}

	result, err := ConfirmUndo(&out, in, info)
	require.NoError(t, err)
	assert.False(t, result)

	output := out.String()
	assert.NotContains(t, output, "Files to revert:")
}

// FixInteractiveMode tests

func TestFixInteractiveMode_ShowsIssuesList(t *testing.T) {
	var out bytes.Buffer
	// Input: 'q' to quit immediately
	in := bytes.NewReader([]byte("q\n"))

	issues := []FixIssue{
		{TaskID: "task-1", Title: "First failed task", Status: "failed", Attempts: 3},
		{TaskID: "task-2", Title: "Blocked task", Status: "blocked", Attempts: 1},
	}
	iterations := []FixIteration{
		{IterationID: "iter001", TaskID: "task-1", Outcome: "failed"},
		{IterationID: "iter002", TaskID: "task-3", Outcome: "success"},
	}

	err := FixInteractiveMode(&out, in, issues, iterations, nil)
	require.NoError(t, err)

	output := out.String()
	// Should show issues
	assert.Contains(t, output, "task-1")
	assert.Contains(t, output, "First failed task")
	assert.Contains(t, output, "failed")
	assert.Contains(t, output, "3") // attempt count
	assert.Contains(t, output, "task-2")
	assert.Contains(t, output, "Blocked task")
	// Should show iterations
	assert.Contains(t, output, "iter001")
	assert.Contains(t, output, "iter002")
}

func TestFixInteractiveMode_RetryCommand(t *testing.T) {
	var out bytes.Buffer
	// Input: 'r task-1' then 'q'
	in := bytes.NewReader([]byte("r task-1\nq\n"))

	issues := []FixIssue{
		{TaskID: "task-1", Title: "Failed task", Status: "failed", Attempts: 2},
	}
	var executedAction *FixAction
	handler := func(action *FixAction) error {
		executedAction = action
		return nil
	}

	err := FixInteractiveMode(&out, in, issues, nil, handler)
	require.NoError(t, err)

	require.NotNil(t, executedAction)
	assert.Equal(t, FixActionRetry, executedAction.Type)
	assert.Equal(t, "task-1", executedAction.TargetID)
	assert.Empty(t, executedAction.Feedback)
}

func TestFixInteractiveMode_SkipCommand(t *testing.T) {
	var out bytes.Buffer
	// Input: 's task-2' then 'q'
	in := bytes.NewReader([]byte("s task-2\nq\n"))

	issues := []FixIssue{
		{TaskID: "task-2", Title: "Blocked task", Status: "blocked", Attempts: 1},
	}
	var executedAction *FixAction
	handler := func(action *FixAction) error {
		executedAction = action
		return nil
	}

	err := FixInteractiveMode(&out, in, issues, nil, handler)
	require.NoError(t, err)

	require.NotNil(t, executedAction)
	assert.Equal(t, FixActionSkip, executedAction.Type)
	assert.Equal(t, "task-2", executedAction.TargetID)
}

func TestFixInteractiveMode_UndoCommand(t *testing.T) {
	var out bytes.Buffer
	// Input: 'u iter001' then 'q'
	in := bytes.NewReader([]byte("u iter001\nq\n"))

	iterations := []FixIteration{
		{IterationID: "iter001", TaskID: "task-1", Outcome: "failed"},
	}
	var executedAction *FixAction
	handler := func(action *FixAction) error {
		executedAction = action
		return nil
	}

	err := FixInteractiveMode(&out, in, nil, iterations, handler)
	require.NoError(t, err)

	require.NotNil(t, executedAction)
	assert.Equal(t, FixActionUndo, executedAction.Type)
	assert.Equal(t, "iter001", executedAction.TargetID)
}

func TestFixInteractiveMode_RetryWithFeedback(t *testing.T) {
	var out bytes.Buffer
	// Input: 'rf task-1' - should trigger editor placeholder
	// For testing, we'll simulate the feedback being provided
	// Since we can't actually open an editor in tests, we'll test that
	// rf triggers the correct action type
	in := bytes.NewReader([]byte("rf task-1\nq\n"))

	issues := []FixIssue{
		{TaskID: "task-1", Title: "Failed task", Status: "failed", Attempts: 2},
	}
	var executedAction *FixAction
	handler := func(action *FixAction) error {
		executedAction = action
		return nil
	}

	// Create a mock editor function for testing
	mockEditor := func(taskID string) (string, error) {
		return "Test feedback from editor", nil
	}

	err := FixInteractiveModeWithEditor(&out, in, issues, nil, handler, mockEditor)
	require.NoError(t, err)

	require.NotNil(t, executedAction)
	assert.Equal(t, FixActionRetry, executedAction.Type)
	assert.Equal(t, "task-1", executedAction.TargetID)
	assert.Equal(t, "Test feedback from editor", executedAction.Feedback)
}

func TestFixInteractiveMode_QuitCommand(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("q\n"))

	var handlerCalled bool
	handler := func(action *FixAction) error {
		handlerCalled = true
		return nil
	}

	err := FixInteractiveMode(&out, in, nil, nil, handler)
	require.NoError(t, err)
	assert.False(t, handlerCalled) // Handler should not be called for quit
}

func TestFixInteractiveMode_InvalidCommand(t *testing.T) {
	var out bytes.Buffer
	// Input: invalid command, then quit
	in := bytes.NewReader([]byte("invalid\nq\n"))

	err := FixInteractiveMode(&out, in, nil, nil, nil)
	require.NoError(t, err)

	output := out.String()
	// Should show error message for invalid command
	assert.Contains(t, output, "Unknown command")
}

func TestFixInteractiveMode_ShowsAttemptCounts(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("q\n"))

	issues := []FixIssue{
		{TaskID: "task-1", Title: "Failed task", Status: "failed", Attempts: 5},
	}

	err := FixInteractiveMode(&out, in, issues, nil, nil)
	require.NoError(t, err)

	output := out.String()
	// Should show attempt count
	assert.Contains(t, output, "5")
}

func TestFixInteractiveMode_ShowsPrompt(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("q\n"))

	err := FixInteractiveMode(&out, in, nil, nil, nil)
	require.NoError(t, err)

	output := out.String()
	// Should show action prompt with available commands
	assert.Contains(t, output, "r <id>")
	assert.Contains(t, output, "s <id>")
	assert.Contains(t, output, "u <id>")
	assert.Contains(t, output, "rf <id>")
	assert.Contains(t, output, "q")
}

// SelectRootTask tests

func TestSelectRootTask_SingleTask(t *testing.T) {
	var out bytes.Buffer
	// No input needed - single task is auto-selected
	in := bytes.NewReader([]byte{})

	tasks := []RootTaskOption{
		{ID: "root-1", Title: "My Feature"},
	}

	selected, err := SelectRootTask(&out, in, tasks, true) // isTTY=true
	require.NoError(t, err)
	assert.Equal(t, "root-1", selected.ID)
	assert.Equal(t, "My Feature", selected.Title)

	// Verify confirmation message
	output := out.String()
	assert.Contains(t, output, "Initializing:")
	assert.Contains(t, output, "My Feature")
	assert.Contains(t, output, "root-1")
}

func TestSelectRootTask_MultipleTasksTTY(t *testing.T) {
	var out bytes.Buffer
	// User selects option 2
	in := bytes.NewReader([]byte("2\n"))

	tasks := []RootTaskOption{
		{ID: "root-1", Title: "Feature One"},
		{ID: "root-2", Title: "Feature Two"},
		{ID: "root-3", Title: "Feature Three"},
	}

	selected, err := SelectRootTask(&out, in, tasks, true) // isTTY=true
	require.NoError(t, err)
	assert.Equal(t, "root-2", selected.ID)
	assert.Equal(t, "Feature Two", selected.Title)

	// Verify menu was displayed
	output := out.String()
	assert.Contains(t, output, "Select a root task")
	assert.Contains(t, output, "1) Feature One")
	assert.Contains(t, output, "2) Feature Two")
	assert.Contains(t, output, "3) Feature Three")
}

func TestSelectRootTask_MultipleTasksNonTTY(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte{})

	tasks := []RootTaskOption{
		{ID: "root-1", Title: "Feature One"},
		{ID: "root-2", Title: "Feature Two"},
	}

	selected, err := SelectRootTask(&out, in, tasks, false) // isTTY=false
	require.Error(t, err)
	assert.Nil(t, selected)
	// Should hint at --parent flag
	assert.Contains(t, err.Error(), "multiple root tasks")
	assert.Contains(t, err.Error(), "--parent")
	// Should list the tasks
	assert.Contains(t, err.Error(), "Feature One")
	assert.Contains(t, err.Error(), "root-1")
}

func TestSelectRootTask_NoTasks(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte{})

	tasks := []RootTaskOption{}

	selected, err := SelectRootTask(&out, in, tasks, true)
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "no tasks")
	assert.Contains(t, err.Error(), "ralph")
}

func TestSelectRootTask_UserCancels(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("q\n"))

	tasks := []RootTaskOption{
		{ID: "root-1", Title: "Feature One"},
		{ID: "root-2", Title: "Feature Two"},
	}

	selected, err := SelectRootTask(&out, in, tasks, true)
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestSelectRootTask_InvalidSelection(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewReader([]byte("99\n"))

	tasks := []RootTaskOption{
		{ID: "root-1", Title: "Feature One"},
		{ID: "root-2", Title: "Feature Two"},
	}

	selected, err := SelectRootTask(&out, in, tasks, true)
	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "invalid selection")
}
