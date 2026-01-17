package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	t.Run("has --config flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.PersistentFlags().Lookup("config")
		require.NotNil(t, flag)
		assert.Equal(t, "ralph.yaml", flag.DefValue)
	})

	t.Run("help shows subcommands", func(t *testing.T) {
		cmd := NewRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "status")
		assert.Contains(t, output, "fix")
	})
}

func TestRootCommand_Flags(t *testing.T) {
	t.Run("has --once/-1 flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("once")
		require.NotNil(t, flag)
		assert.Equal(t, "false", flag.DefValue)
		assert.Equal(t, "1", flag.Shorthand)
	})

	t.Run("has --max-iterations/-n flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("max-iterations")
		require.NotNil(t, flag)
		assert.Equal(t, "0", flag.DefValue)
	})

	t.Run("has --parent/-p flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("parent")
		require.NotNil(t, flag)
	})

	t.Run("has --branch/-b flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("branch")
		require.NotNil(t, flag)
	})

	t.Run("has --dry-run flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("dry-run")
		require.NotNil(t, flag)
		assert.Equal(t, "false", flag.DefValue)
	})

	t.Run("accepts optional file argument", func(t *testing.T) {
		cmd := NewRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "[file]")
	})
}

func TestRootCommand_FileValidation(t *testing.T) {
	t.Run("errors on non-existent file", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"/nonexistent/path/file.md"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("errors on unknown file type", func(t *testing.T) {
		tmpDir := t.TempDir()
		unknownPath := filepath.Join(tmpDir, "random.txt")
		require.NoError(t, os.WriteFile(unknownPath, []byte("Hello world"), 0644))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{"--dry-run", unknownPath})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown file type")
	})

	t.Run("dry-run detects PRD file", func(t *testing.T) {
		tmpDir := t.TempDir()
		prdPath := filepath.Join(tmpDir, "prd.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# Product\n\n## Objectives\n\nBuild something."), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--dry-run", prdPath})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, out.String(), "[dry-run]")
		assert.Contains(t, out.String(), "PRD")
	})

	t.Run("dry-run detects task YAML file", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "tasks.yaml")
		require.NoError(t, os.WriteFile(yamlPath, []byte("id: task-001\ntitle: Do something\nstatus: open"), 0644))

		cmd := NewRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--dry-run", yamlPath})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, out.String(), "[dry-run]")
		assert.Contains(t, out.String(), "task")
	})
}

func TestRootCommand_NoTasks(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tasks")
}
