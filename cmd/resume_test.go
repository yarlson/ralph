package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResumeCommand_Structure(t *testing.T) {
	cmd := newResumeCmd()

	assert.Equal(t, "resume", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestResumeCommand_NoRalphDir(t *testing.T) {
	// Create temp dir without .ralph
	tmpDir := t.TempDir()

	// Change to temp dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"resume"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".ralph")
}

func TestResumeCommand_NotPaused(t *testing.T) {
	// Set up test directory
	tmpDir := t.TempDir()

	// Create .ralph/state directory but no paused file
	stateDir := filepath.Join(tmpDir, ".ralph", "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))

	// Change to temp dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"resume"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Should succeed but indicate not paused
	assert.Contains(t, out.String(), "not paused")
}

func TestResumeCommand_RemovesPausedFlag(t *testing.T) {
	// Set up test directory
	tmpDir := t.TempDir()

	// Create .ralph/state directory and paused file
	stateDir := filepath.Join(tmpDir, ".ralph", "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	pausedFile := filepath.Join(stateDir, "paused")
	require.NoError(t, os.WriteFile(pausedFile, []byte{}, 0644))

	// Verify paused file exists
	_, err := os.Stat(pausedFile)
	require.NoError(t, err)

	// Change to temp dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"resume"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Check that paused file was removed
	_, err = os.Stat(pausedFile)
	assert.True(t, os.IsNotExist(err), "paused file should be removed")

	// Check output
	assert.Contains(t, out.String(), "resumed")
}

func TestResumeCommand_ShowsDeprecationWarning(t *testing.T) {
	// Set up test directory
	tmpDir := t.TempDir()

	// Create .ralph/state directory and paused file
	stateDir := filepath.Join(tmpDir, ".ralph", "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	pausedFile := filepath.Join(stateDir, "paused")
	require.NoError(t, os.WriteFile(pausedFile, []byte{}, 0644))

	// Change to temp dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"resume"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Should show deprecation warning
	assert.Contains(t, out.String(), "Deprecated:")
	assert.Contains(t, out.String(), "ralph")

	// Should still execute (paused file removed)
	_, err = os.Stat(pausedFile)
	assert.True(t, os.IsNotExist(err), "paused file should be removed after deprecation warning")
}
