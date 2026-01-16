package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_WithValidFile(t *testing.T) {
	// Create a temp directory with a ralph.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	configContent := `
repo:
  root: .
  branch_prefix: "ralph/"
tasks:
  backend: "local"
  path: ".ralph/tasks"
  parent_id_file: ".ralph/parent-task-id"
memory:
  progress_file: ".ralph/progress.md"
  archive_dir: ".ralph/archive"
claude:
  command: ["claude", "code"]
  args: ["--verbose"]
verification:
  commands:
    - ["go", "build", "./..."]
    - ["go", "test", "./..."]
loop:
  max_iterations: 50
  max_minutes_per_iteration: 20
  gutter:
    max_same_failure: 3
    max_churn_commits: 2
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

	// Repo
	assert.Equal(t, ".", cfg.Repo.Root)
	assert.Equal(t, "ralph/", cfg.Repo.BranchPrefix)

	// Tasks
	assert.Equal(t, "local", cfg.Tasks.Backend)
	assert.Equal(t, ".ralph/tasks", cfg.Tasks.Path)
	assert.Equal(t, ".ralph/parent-task-id", cfg.Tasks.ParentIDFile)

	// Memory
	assert.Equal(t, ".ralph/progress.md", cfg.Memory.ProgressFile)
	assert.Equal(t, ".ralph/archive", cfg.Memory.ArchiveDir)

	// Claude
	assert.Equal(t, []string{"claude", "code"}, cfg.Claude.Command)
	assert.Equal(t, []string{"--verbose"}, cfg.Claude.Args)

	// Verification
	assert.Len(t, cfg.Verification.Commands, 2)
	assert.Equal(t, []string{"go", "build", "./..."}, cfg.Verification.Commands[0])
	assert.Equal(t, []string{"go", "test", "./..."}, cfg.Verification.Commands[1])

	// Loop
	assert.Equal(t, 50, cfg.Loop.MaxIterations)
	assert.Equal(t, 20, cfg.Loop.MaxMinutesPerIteration)
	assert.Equal(t, 3, cfg.Loop.Gutter.MaxSameFailure)
	assert.Equal(t, 2, cfg.Loop.Gutter.MaxChurnCommits)

	// Safety
	assert.True(t, cfg.Safety.Sandbox)
	assert.Equal(t, []string{"go", "git"}, cfg.Safety.AllowedCommands)
}

func TestLoadConfig_WithDefaults(t *testing.T) {
	// Create a temp directory without ralph.yaml
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	require.NoError(t, err)

	// Check defaults are sensible
	assert.Equal(t, ".", cfg.Repo.Root)
	assert.Equal(t, "ralph/", cfg.Repo.BranchPrefix)

	assert.Equal(t, "local", cfg.Tasks.Backend)
	assert.Equal(t, ".ralph/tasks", cfg.Tasks.Path)
	assert.Equal(t, ".ralph/parent-task-id", cfg.Tasks.ParentIDFile)

	assert.Equal(t, ".ralph/progress.md", cfg.Memory.ProgressFile)
	assert.Equal(t, ".ralph/archive", cfg.Memory.ArchiveDir)

	assert.Equal(t, []string{"claude"}, cfg.Claude.Command)
	assert.Empty(t, cfg.Claude.Args)

	assert.Empty(t, cfg.Verification.Commands)

	assert.Equal(t, 50, cfg.Loop.MaxIterations)
	assert.Equal(t, 20, cfg.Loop.MaxMinutesPerIteration)
	assert.Equal(t, 3, cfg.Loop.Gutter.MaxSameFailure)
	assert.Equal(t, 2, cfg.Loop.Gutter.MaxChurnCommits)

	assert.False(t, cfg.Safety.Sandbox)
	assert.Empty(t, cfg.Safety.AllowedCommands)
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	// Only override some values
	configContent := `
loop:
  max_iterations: 100
safety:
  sandbox: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(tmpDir)
	require.NoError(t, err)

	// Overridden values
	assert.Equal(t, 100, cfg.Loop.MaxIterations)
	assert.True(t, cfg.Safety.Sandbox)

	// Default values should still be present
	assert.Equal(t, ".", cfg.Repo.Root)
	assert.Equal(t, "local", cfg.Tasks.Backend)
	assert.Equal(t, 20, cfg.Loop.MaxMinutesPerIteration)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ralph.yaml")

	invalidContent := `
repo:
  root: [invalid
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(tmpDir)
	assert.Error(t, err)
}

func TestConfig_RepoRoot(t *testing.T) {
	cfg := &Config{
		Repo: RepoConfig{
			Root: ".",
		},
	}
	assert.Equal(t, ".", cfg.Repo.Root)
}
