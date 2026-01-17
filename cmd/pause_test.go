package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPauseCommand_Structure(t *testing.T) {
	cmd := newPauseCmd()

	assert.Equal(t, "pause", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestPauseCommand_NoRalphDir(t *testing.T) {
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
	cmd.SetArgs([]string{"pause"})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".ralph")
}

func TestPauseCommand_SetsPausedFlag(t *testing.T) {
	// Set up test directory
	tmpDir := t.TempDir()

	// Create .ralph/state directory
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
	cmd.SetArgs([]string{"pause"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Check that paused file was created
	pausedFile := filepath.Join(stateDir, "paused")
	_, err = os.Stat(pausedFile)
	assert.NoError(t, err, "paused file should exist")

	// Check output
	assert.Contains(t, out.String(), "paused")
}

func TestPauseCommand_AlreadyPaused(t *testing.T) {
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
	cmd.SetArgs([]string{"pause"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Should still succeed but indicate already paused
	assert.Contains(t, out.String(), "already paused")
}

func TestPauseCommand_ShowsDeprecationWarning(t *testing.T) {
	// Set up test directory
	tmpDir := t.TempDir()

	// Create .ralph/state directory
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
	cmd.SetArgs([]string{"pause"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Should show deprecation warning
	assert.Contains(t, out.String(), "Deprecated:")
	assert.Contains(t, out.String(), "Ctrl+C to stop")

	// Should still execute (paused file created)
	pausedFile := filepath.Join(stateDir, "paused")
	_, err = os.Stat(pausedFile)
	assert.NoError(t, err, "paused file should exist after deprecation warning")
}
