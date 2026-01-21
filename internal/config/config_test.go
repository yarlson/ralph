package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromPath_WithValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.yaml")

	configContent := `
provider: "opencode"
opencode:
  args: ["--model", "openai/gpt-4o-mini"]
safety:
  sandbox: true
  allowed_commands: ["npm", "go"]
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfigFromPath(configPath)
	require.NoError(t, err)

	assert.Equal(t, "opencode", cfg.Provider)
	assert.Equal(t, []string{"opencode", "run"}, cfg.OpenCode.Command)
	assert.Equal(t, []string{"--model", "openai/gpt-4o-mini"}, cfg.OpenCode.Args)
	assert.True(t, cfg.Safety.Sandbox)
	assert.Equal(t, []string{"npm", "go"}, cfg.Safety.AllowedCommands)
}

func TestLoadConfigFromPath_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	cfg, err := LoadConfigFromPath(configPath)
	require.NoError(t, err)

	assert.Equal(t, "claude", cfg.Provider)
	assert.False(t, cfg.Safety.Sandbox)
}

func TestLoadConfigFromPath_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `
provider: [invalid
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = LoadConfigFromPath(configPath)
	assert.Error(t, err)
}

func TestLoadConfigWithFile_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "my-config.yaml")

	configContent := `
provider: "opencode"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfigWithFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, "opencode", cfg.Provider)
}

func TestLoadConfigWithFile_GlobalFallback(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalPath := filepath.Join(globalDir, "ralph", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0755))
	require.NoError(t, os.WriteFile(globalPath, []byte("provider: \"opencode\"\n"), 0644))

	cfg, err := LoadConfigWithFile("")
	require.NoError(t, err)

	assert.Equal(t, "opencode", cfg.Provider)
}

func TestLoadConfigWithFile_NoConfigDefaults(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)

	cfg, err := LoadConfigWithFile("")
	require.NoError(t, err)

	assert.Equal(t, "claude", cfg.Provider)
}

func TestConfig_SandboxMode(t *testing.T) {
	t.Run("sandbox disabled by default", func(t *testing.T) {
		cfg, err := LoadConfigWithFile("")
		require.NoError(t, err)

		assert.False(t, cfg.Safety.Sandbox)
		assert.Equal(t, []string{"npm", "go", "git"}, cfg.Safety.AllowedCommands)
	})

	t.Run("sandbox can be enabled with custom allowlist", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "ralph.yaml")

		configContent := `
safety:
  sandbox: true
  allowed_commands: ["go", "npm"]
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := LoadConfigFromPath(configPath)
		require.NoError(t, err)

		assert.True(t, cfg.Safety.Sandbox)
		assert.Equal(t, []string{"go", "npm"}, cfg.Safety.AllowedCommands)
	})

	t.Run("sandbox enabled with empty allowlist", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "ralph.yaml")

		configContent := `
safety:
  sandbox: true
  allowed_commands: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := LoadConfigFromPath(configPath)
		require.NoError(t, err)

		assert.True(t, cfg.Safety.Sandbox)
		assert.Empty(t, cfg.Safety.AllowedCommands)
	})
}
