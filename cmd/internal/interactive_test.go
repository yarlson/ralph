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
