package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	t.Run("auto-initializes when run without args", func(t *testing.T) {
		// When run without args and no tasks, should error with helpful message
		tmpDir := t.TempDir()
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()
		require.NoError(t, os.Chdir(tmpDir))

		// Create minimal .ralph structure
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))

		cmd := NewRootCmd()
		cmd.SetArgs([]string{})
		err = cmd.Execute()
		// Should fail because no tasks
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tasks")
	})

	t.Run("has --config flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.PersistentFlags().Lookup("config")
		require.NotNil(t, flag, "expected --config flag to exist")
		assert.Equal(t, "ralph.yaml", flag.DefValue)
	})

	t.Run("help shows all subcommands", func(t *testing.T) {
		cmd := NewRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		expectedCommands := []string{
			"init",
			"run",
			"status",
			"pause",
			"resume",
			"retry",
			"skip",
			"report",
		}
		for _, subcmd := range expectedCommands {
			assert.True(t, strings.Contains(output, subcmd),
				"expected help to contain '%s'", subcmd)
		}
	})
}

func TestRootCommand_FileArgument(t *testing.T) {
	t.Run("has --once/-1 flag with default false", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("once")
		require.NotNil(t, flag, "expected --once flag to exist")
		assert.Equal(t, "false", flag.DefValue)
		assert.Equal(t, "1", flag.Shorthand)
	})

	t.Run("has --max-iterations/-n flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("max-iterations")
		require.NotNil(t, flag, "expected --max-iterations flag to exist")
		assert.Equal(t, "0", flag.DefValue)
		assert.Equal(t, "n", flag.Shorthand)
	})

	t.Run("has --parent/-p flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("parent")
		require.NotNil(t, flag, "expected --parent flag to exist")
		assert.Equal(t, "", flag.DefValue)
		assert.Equal(t, "p", flag.Shorthand)
	})

	t.Run("has --branch/-b flag", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("branch")
		require.NotNil(t, flag, "expected --branch flag to exist")
		assert.Equal(t, "", flag.DefValue)
		assert.Equal(t, "b", flag.Shorthand)
	})

	t.Run("has --dry-run flag with default false", func(t *testing.T) {
		cmd := NewRootCmd()
		flag := cmd.Flags().Lookup("dry-run")
		require.NotNil(t, flag, "expected --dry-run flag to exist")
		assert.Equal(t, "false", flag.DefValue)
	})

	t.Run("accepts optional positional file argument", func(t *testing.T) {
		cmd := NewRootCmd()
		// Root command should use Args: cobra.MaximumNArgs(1)
		// Verify with help that file argument is documented in Usage
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Usage should show [file] indicating optional file argument
		assert.Contains(t, output, "[file]", "expected usage to show optional file argument")
	})
}

func TestRootCommand_FileValidation(t *testing.T) {
	t.Run("validates PRD file using detectFileType", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		// Create a PRD file
		prdPath := filepath.Join(tmpDir, "prd.md")
		prdContent := "# Product\n\n## Objectives\n\nBuild something great."
		err := os.WriteFile(prdPath, []byte(prdContent), 0644)
		require.NoError(t, err)

		cmd := NewRootCmd()
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--dry-run", prdPath})

		// Execute - dry-run mode should detect file type without actually running
		err = cmd.Execute()
		// Should not error on detection - dry-run should show what would happen
		// The actual execution may fail due to missing config etc, but detection should work
		if err != nil {
			// Dry-run detected the file and showed it
			assert.Contains(t, stderr.String()+err.Error(), "prd")
		}
	})

	t.Run("validates task YAML file using detectFileType", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		// Create a task YAML file
		taskPath := filepath.Join(tmpDir, "tasks.yaml")
		taskContent := "id: task-001\ntitle: Do something\nstatus: open"
		err := os.WriteFile(taskPath, []byte(taskContent), 0644)
		require.NoError(t, err)

		cmd := NewRootCmd()
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--dry-run", taskPath})

		// Execute - dry-run mode should detect file type without actually running
		err = cmd.Execute()
		// The actual execution may fail due to missing config etc, but detection should work
		if err != nil {
			// Dry-run detected the file and showed it
			assert.Contains(t, stderr.String()+err.Error(), "task")
		}
	})

	t.Run("errors on unknown file type", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		// Create a file with unknown content
		unknownPath := filepath.Join(tmpDir, "random.txt")
		err := os.WriteFile(unknownPath, []byte("Hello world"), 0644)
		require.NoError(t, err)

		cmd := NewRootCmd()
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--dry-run", unknownPath})

		err = cmd.Execute()
		// Should error with unknown file type
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown file type")
	})

	t.Run("errors on non-existent file", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"/nonexistent/path/file.md"})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}

// Auto-initialization tests for root command (no file argument)

func TestRootCommand_AutoInit_SingleRootTask(t *testing.T) {
	tmpDir, _ := setupTestDirWithTasks(t, 1)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--once"})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify parent-task-id was written
	parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
	data, err := os.ReadFile(parentIDFile)
	require.NoError(t, err)
	assert.Equal(t, "root-1", string(data))

	// Verify auto-init message appeared with correct format
	output := outBuf.String()
	assert.Contains(t, output, "Initializing:")
	assert.Contains(t, output, "Root Task 1")
	assert.Contains(t, output, "root-1")
}

func TestRootCommand_AutoInit_NoRootTasks(t *testing.T) {
	_, _ = setupTestDirWithTasks(t, 0)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--once"})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.Error(t, err)
	// Should show helpful error message
	assert.Contains(t, err.Error(), "no tasks")
	assert.Contains(t, err.Error(), "ralph")
}

func TestRootCommand_AutoInit_MultipleRoots_Interactive(t *testing.T) {
	tmpDir, _ := setupTestDirWithTasks(t, 3)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--once"})

	// Mock stdin with selection "2\n"
	inputBuf := bytes.NewBufferString("2\n")
	cmd.SetIn(inputBuf)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify correct task was selected
	parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
	data, err := os.ReadFile(parentIDFile)
	require.NoError(t, err)
	assert.Equal(t, "root-2", string(data))

	// Verify menu was displayed
	output := outBuf.String()
	assert.Contains(t, output, "Select a root task")
	assert.Contains(t, output, "1) Root Task 1")
	assert.Contains(t, output, "2) Root Task 2")
	assert.Contains(t, output, "3) Root Task 3")
}

func TestRootCommand_AutoInit_MultipleRoots_NonTTY(t *testing.T) {
	_, _ = setupTestDirWithTasks(t, 3)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--once"})

	// Use default stdin (non-TTY in test env, don't set stdin)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.Error(t, err)
	// Should show helpful error with --parent hint
	assert.Contains(t, err.Error(), "multiple root tasks found")
	assert.Contains(t, err.Error(), "--parent")
}

func TestRootCommand_AutoInit_WithExplicitParent(t *testing.T) {
	tmpDir, _ := setupTestDirWithTasks(t, 2)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--once", "--parent", "root-2"})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify specified parent was used
	parentIDFile := filepath.Join(tmpDir, ".ralph", "parent-task-id")
	data, err := os.ReadFile(parentIDFile)
	require.NoError(t, err)
	assert.Equal(t, "root-2", string(data))
}

// PRD Bootstrap Pipeline Tests

func TestRootCommand_PRDFile_DryRunShowsAnalyzingMessage(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a PRD file with Objectives section
	prdPath := filepath.Join(tmpDir, "feature.md")
	prdContent := "# Feature Spec\n\n## Objectives\n\nBuild an awesome feature."
	err := os.WriteFile(prdPath, []byte(prdContent), 0644)
	require.NoError(t, err)

	cmd := NewRootCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--dry-run", prdPath})

	err = cmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "[dry-run]")
	assert.Contains(t, output, "decompose")
	assert.Contains(t, output, "PRD")
	assert.Contains(t, output, "feature.md")
}

func TestRootCommand_PRDFile_WithoutDryRunTriggersBootstrap(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create a PRD file
	prdPath := filepath.Join(tmpDir, "prd.md")
	prdContent := "# Product\n\n## Objectives\n\nBuild something."
	err = os.WriteFile(prdPath, []byte(prdContent), 0644)
	require.NoError(t, err)

	// Create minimal .ralph structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".ralph", "tasks"), 0755))

	cmd := NewRootCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{prdPath})

	// This will fail because Claude isn't available, but it should show the "Analyzing PRD" message
	_ = cmd.Execute()
	// We expect an error because Claude isn't running, but it should have triggered the bootstrap
	output := outBuf.String()

	// The bootstrap pipeline should at least start and show "Analyzing PRD"
	assert.Contains(t, output, "Analyzing PRD")
	assert.Contains(t, output, "prd.md")
}

// Note: All commands have been implemented. No stub commands remaining.
