package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecomposeCmd_Structure(t *testing.T) {
	cmd := newDecomposeCmd()

	assert.Equal(t, "decompose <prd-file>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestDecomposeCmd_Flags(t *testing.T) {
	cmd := newDecomposeCmd()

	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag, "--output flag should be defined")
	assert.Equal(t, "", outputFlag.DefValue, "--output should default to empty")

	importFlag := cmd.Flags().Lookup("import")
	require.NotNil(t, importFlag, "--import flag should be defined")
	assert.Equal(t, "false", importFlag.DefValue, "--import should default to false")

	timeoutFlag := cmd.Flags().Lookup("timeout")
	require.NotNil(t, timeoutFlag, "--timeout flag should be defined")
	assert.Equal(t, "300", timeoutFlag.DefValue, "--timeout should default to 300")
}

func TestDecomposeCmd_NoArgs(t *testing.T) {
	cmd := newDecomposeCmd()
	cmd.SetArgs([]string{})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a PRD file path")
}

func TestDecomposeCmd_FileDoesNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	// Create .ralph directory structure
	ralphDir := filepath.Join(tmpDir, ".ralph")
	require.NoError(t, os.MkdirAll(ralphDir, 0755))

	cmd := newDecomposeCmd()
	cmd.SetArgs([]string{"nonexistent-prd.md"})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PRD file not found")
}

func TestDecomposeCmd_ShortOutputFlag(t *testing.T) {
	cmd := newDecomposeCmd()

	// Test that -o flag is the short form of --output
	outputFlag := cmd.Flags().ShorthandLookup("o")
	require.NotNil(t, outputFlag, "-o flag should be defined as shorthand")
	assert.Equal(t, "output", outputFlag.Name)
}

func TestDecomposeCmd_DefaultOutput(t *testing.T) {
	// This is a simple structure test - we don't run the actual command
	// because it would call Claude Code
	cmd := newDecomposeCmd()

	// Verify command structure
	assert.Contains(t, cmd.Long, "task.yaml")
	assert.Contains(t, cmd.Long, "PRD")
	assert.Contains(t, cmd.Long, "specification")
}

func TestDecomposeCmd_Examples(t *testing.T) {
	cmd := newDecomposeCmd()

	// Verify examples are in the long description
	assert.Contains(t, cmd.Long, "ralph decompose docs/prd.md")
	assert.Contains(t, cmd.Long, "--output")
	assert.Contains(t, cmd.Long, "--import")
}
