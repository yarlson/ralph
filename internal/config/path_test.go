package config

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalConfigPath_UsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")

	path, err := GlobalConfigPath()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("/tmp/xdg", "ralph", "config.yaml"), path)
}

func TestGlobalConfigPath_DefaultsToHomeDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	originalUserHomeDir := userHomeDir
	t.Cleanup(func() {
		userHomeDir = originalUserHomeDir
	})

	userHomeDir = func() (string, error) {
		return "/home/ralph", nil
	}

	path, err := GlobalConfigPath()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("/home/ralph", ".config", "ralph", "config.yaml"), path)
}

func TestGlobalConfigPath_HomeDirError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	originalUserHomeDir := userHomeDir
	t.Cleanup(func() {
		userHomeDir = originalUserHomeDir
	})

	sentinelErr := errors.New("home dir unavailable")
	userHomeDir = func() (string, error) {
		return "", sentinelErr
	}

	_, err := GlobalConfigPath()
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinelErr)
}
