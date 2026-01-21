package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_WithValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	configContent := `
provider: "opencode"
claude:
  command: ["claude", "code"]
  args: ["--verbose"]
opencode:
  command: ["opencode", "run"]
  args: ["--model", "openai/gpt-4o-mini"]
safety:
  sandbox: true
  allowed_commands:
    - "go"
    - "git"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "opencode", cfg.Provider)
	assert.Equal(t, []string{"claude", "code"}, cfg.Claude.Command)
	assert.Equal(t, []string{"--verbose"}, cfg.Claude.Args)
	assert.Equal(t, []string{"opencode", "run"}, cfg.OpenCode.Command)
	assert.Equal(t, []string{"--model", "openai/gpt-4o-mini"}, cfg.OpenCode.Args)
	assert.True(t, cfg.Safety.Sandbox)
	assert.Equal(t, []string{"go", "git"}, cfg.Safety.AllowedCommands)
}

func TestLoadConfig_WithDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "claude", cfg.Provider)
	assert.Equal(t, []string{"claude"}, cfg.Claude.Command)
	assert.Empty(t, cfg.Claude.Args)
	assert.Equal(t, []string{"opencode", "run"}, cfg.OpenCode.Command)
	assert.Empty(t, cfg.OpenCode.Args)
	assert.False(t, cfg.Safety.Sandbox)
	assert.Equal(t, []string{"npm", "go", "git"}, cfg.Safety.AllowedCommands)
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	configContent := `
safety:
  sandbox: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(tmpDir)
	require.NoError(t, err)

	assert.True(t, cfg.Safety.Sandbox)
	assert.Equal(t, "claude", cfg.Provider)
	assert.Equal(t, []string{"claude"}, cfg.Claude.Command)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	invalidContent := `
provider: [invalid
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(tmpDir)
	assert.Error(t, err)
}

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

	cfg, err := LoadConfigWithFile(tmpDir, configPath)
	require.NoError(t, err)

	assert.Equal(t, "opencode", cfg.Provider)
}

func TestLoadConfigWithFile_WithEmptyConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	configContent := `
provider: "opencode"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfigWithFile(tmpDir, "")
	require.NoError(t, err)

	assert.Equal(t, "opencode", cfg.Provider)
}

func TestLoadConfigWithFile_FallbackToDefault(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadConfigWithFile(tmpDir, "")
	require.NoError(t, err)

	assert.Equal(t, "claude", cfg.Provider)
}

func TestConfig_SandboxMode(t *testing.T) {
	t.Run("sandbox disabled by default", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg, err := LoadConfig(tmpDir)
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

		cfg, err := LoadConfig(tmpDir)
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

		cfg, err := LoadConfig(tmpDir)
		require.NoError(t, err)

		assert.True(t, cfg.Safety.Sandbox)
		assert.Empty(t, cfg.Safety.AllowedCommands)
	})
}
