package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportCmd_Structure(t *testing.T) {
	cmd := newImportCmd()

	assert.Equal(t, "import <file>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestImportCmd_Flags(t *testing.T) {
	cmd := newImportCmd()

	overwriteFlag := cmd.Flags().Lookup("overwrite")
	require.NotNil(t, overwriteFlag, "--overwrite flag should be defined")
	assert.Equal(t, "false", overwriteFlag.DefValue, "--overwrite should default to false")
}

func TestImportCmd_NoArgs(t *testing.T) {
	cmd := newImportCmd()
	cmd.SetArgs([]string{})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a YAML file path")
}

func TestImportCmd_FileDoesNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	cmd := newImportCmd()
	cmd.SetArgs([]string{"nonexistent.yaml"})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestImportCmd_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	// Create invalid YAML file
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("invalid: yaml: content:"), 0644))

	cmd := newImportCmd()
	cmd.SetArgs([]string{yamlFile})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestImportCmd_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	// Create valid YAML file
	yamlContent := `tasks:
  - id: task-1
    title: Test Task 1
    description: A test task
  - id: task-2
    title: Test Task 2
    description: Another test task
`
	yamlFile := filepath.Join(tmpDir, "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	cmd := newImportCmd()
	cmd.SetArgs([]string{yamlFile})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Successfully imported 2 task(s)")
}

func TestImportCmd_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	// Create YAML with validation errors (missing required fields)
	yamlContent := `tasks:
  - id: ""
    title: ""
  - id: task-2
    title: Test Task 2
`
	yamlFile := filepath.Join(tmpDir, "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	cmd := newImportCmd()
	cmd.SetArgs([]string{yamlFile})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	require.NoError(t, err) // Should succeed but report errors

	output := outBuf.String()
	assert.Contains(t, output, "Successfully imported 1 task(s)")
	assert.Contains(t, output, "1 error(s) occurred")
}

func TestImportCmd_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	// First import
	yamlContent := `tasks:
  - id: task-1
    title: Original Title
    status: open
`
	yamlFile := filepath.Join(tmpDir, "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	cmd1 := newImportCmd()
	cmd1.SetArgs([]string{yamlFile})
	require.NoError(t, cmd1.Execute())

	// Second import with updated content
	yamlContent2 := `tasks:
  - id: task-1
    title: Updated Title
    status: completed
`
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent2), 0644))

	cmd2 := newImportCmd()
	cmd2.SetArgs([]string{yamlFile})

	var outBuf bytes.Buffer
	cmd2.SetOut(&outBuf)

	err := cmd2.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Successfully imported 1 task(s)")
}

func TestImportCmd_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	// Create empty YAML file
	yamlFile := filepath.Join(tmpDir, "empty.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(""), 0644))

	cmd := newImportCmd()
	cmd.SetArgs([]string{yamlFile})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Successfully imported 0 task(s)")
}

func TestImportCmd_AtomicImport(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph", "tasks")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	// Create YAML with mix of valid and invalid tasks
	yamlContent := `tasks:
  - id: task-1
    title: Valid Task
  - id: ""
    title: ""
  - id: task-2
    title: Another Valid Task
`
	yamlFile := filepath.Join(tmpDir, "tasks.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0644))

	cmd := newImportCmd()
	cmd.SetArgs([]string{yamlFile})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	// Import is not atomic - valid tasks are imported, invalid tasks are skipped
	assert.Contains(t, output, "Successfully imported 2 task(s)")
	assert.Contains(t, output, "1 error(s) occurred")
}
