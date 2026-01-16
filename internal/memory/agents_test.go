package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindAgentsMd_NoFiles(t *testing.T) {
	dir := t.TempDir()

	files, err := FindAgentsMd(dir)
	require.NoError(t, err)
	assert.Empty(t, files, "should find no files in empty directory")
}

func TestFindAgentsMd_SingleFile(t *testing.T) {
	dir := t.TempDir()

	// Create AGENTS.md in root
	agentsPath := filepath.Join(dir, "AGENTS.md")
	err := os.WriteFile(agentsPath, []byte("# Agents\nSome content"), 0644)
	require.NoError(t, err)

	files, err := FindAgentsMd(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1, "should find one file")
	assert.Equal(t, agentsPath, files[0])
}

func TestFindAgentsMd_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// Create directory structure
	// root/AGENTS.md
	// root/internal/AGENTS.md
	// root/cmd/AGENTS.md

	rootAgents := filepath.Join(dir, "AGENTS.md")
	err := os.WriteFile(rootAgents, []byte("root"), 0644)
	require.NoError(t, err)

	internalDir := filepath.Join(dir, "internal")
	err = os.MkdirAll(internalDir, 0755)
	require.NoError(t, err)
	internalAgents := filepath.Join(internalDir, "AGENTS.md")
	err = os.WriteFile(internalAgents, []byte("internal"), 0644)
	require.NoError(t, err)

	cmdDir := filepath.Join(dir, "cmd")
	err = os.MkdirAll(cmdDir, 0755)
	require.NoError(t, err)
	cmdAgents := filepath.Join(cmdDir, "AGENTS.md")
	err = os.WriteFile(cmdAgents, []byte("cmd"), 0644)
	require.NoError(t, err)

	files, err := FindAgentsMd(dir)
	require.NoError(t, err)
	assert.Len(t, files, 3, "should find three files")
	assert.Contains(t, files, rootAgents)
	assert.Contains(t, files, internalAgents)
	assert.Contains(t, files, cmdAgents)
}

func TestFindAgentsMd_IgnoresNonAgentsFiles(t *testing.T) {
	dir := t.TempDir()

	// Create various files
	err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "AGENTS.txt"), []byte("txt"), 0644)
	require.NoError(t, err)

	files, err := FindAgentsMd(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1, "should only find AGENTS.md")
}

func TestReadAgentsMd_NoFiles(t *testing.T) {
	dir := t.TempDir()

	content, err := ReadAgentsMd(dir)
	require.NoError(t, err)
	assert.Empty(t, content, "should return empty string when no files found")
}

func TestReadAgentsMd_SingleFile(t *testing.T) {
	dir := t.TempDir()

	agentsPath := filepath.Join(dir, "AGENTS.md")
	expectedContent := "# Agents\nSome patterns"
	err := os.WriteFile(agentsPath, []byte(expectedContent), 0644)
	require.NoError(t, err)

	content, err := ReadAgentsMd(dir)
	require.NoError(t, err)
	assert.Contains(t, content, expectedContent)
	assert.Contains(t, content, agentsPath, "should include file path")
}

func TestReadAgentsMd_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// Create two AGENTS.md files
	rootAgents := filepath.Join(dir, "AGENTS.md")
	err := os.WriteFile(rootAgents, []byte("root patterns"), 0644)
	require.NoError(t, err)

	subDir := filepath.Join(dir, "internal")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	internalAgents := filepath.Join(subDir, "AGENTS.md")
	err = os.WriteFile(internalAgents, []byte("internal patterns"), 0644)
	require.NoError(t, err)

	content, err := ReadAgentsMd(dir)
	require.NoError(t, err)
	assert.Contains(t, content, "root patterns")
	assert.Contains(t, content, "internal patterns")
	assert.Contains(t, content, rootAgents)
	assert.Contains(t, content, internalAgents)
}

func TestReadAgentsMd_TruncatesLargeFiles(t *testing.T) {
	dir := t.TempDir()

	agentsPath := filepath.Join(dir, "AGENTS.md")
	// Create a file larger than 10KB
	largeContent := make([]byte, 15*1024)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	err := os.WriteFile(agentsPath, largeContent, 0644)
	require.NoError(t, err)

	content, err := ReadAgentsMd(dir)
	require.NoError(t, err)
	assert.Contains(t, content, "[truncated]", "should truncate large files")
	// Total content should not exceed approximately MaxAgentsBytes per file
	// (exact size depends on formatting, but should be reasonable)
	assert.Less(t, len(content), 20*1024, "content should be truncated")
}

func TestReadAgentsMd_InvalidPath(t *testing.T) {
	content, err := ReadAgentsMd("/nonexistent/path/that/does/not/exist")
	require.NoError(t, err, "should not error on nonexistent path")
	assert.Empty(t, content, "should return empty string for invalid path")
}
