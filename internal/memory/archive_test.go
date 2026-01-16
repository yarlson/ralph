package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressArchive_NewProgressArchive(t *testing.T) {
	archiveDir := "/test/archive"
	archive := NewProgressArchive(archiveDir)

	assert.NotNil(t, archive)
	assert.Equal(t, archiveDir, archive.archiveDir)
}

func TestProgressArchive_Archive_MovesFileCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	progressPath := filepath.Join(tmpDir, "progress.md")
	archiveDir := filepath.Join(tmpDir, "archive")

	// Create a progress file with content
	content := `# Ralph MVP Progress

**Feature**: Test Feature
**Parent Task**: test-task
**Started**: 2026-01-15

---

## Codebase Patterns

- Pattern 1

---

## Iteration Log

### 2026-01-15: task-1 (Task One)

**What changed:**
- Did something

**Outcome**: Success
`
	err := os.WriteFile(progressPath, []byte(content), 0644)
	require.NoError(t, err)

	archive := NewProgressArchive(archiveDir)
	archivedPath, err := archive.Archive(progressPath)
	require.NoError(t, err)

	// Original file should no longer exist
	_, err = os.Stat(progressPath)
	assert.True(t, os.IsNotExist(err), "original file should be removed")

	// Archived file should exist in archive directory
	assert.DirExists(t, archiveDir)
	assert.FileExists(t, archivedPath)

	// Archived file should have correct content
	archivedContent, err := os.ReadFile(archivedPath)
	require.NoError(t, err)
	assert.Equal(t, content, string(archivedContent))
}

func TestProgressArchive_Archive_TimestampInFilename(t *testing.T) {
	tmpDir := t.TempDir()
	progressPath := filepath.Join(tmpDir, "progress.md")
	archiveDir := filepath.Join(tmpDir, "archive")

	err := os.WriteFile(progressPath, []byte("test content"), 0644)
	require.NoError(t, err)

	archive := NewProgressArchive(archiveDir)
	archivedPath, err := archive.Archive(progressPath)
	require.NoError(t, err)

	// Filename should contain timestamp in format: progress-YYYYMMDD-HHMMSS.md
	filename := filepath.Base(archivedPath)
	assert.True(t, strings.HasPrefix(filename, "progress-"), "filename should start with 'progress-'")
	assert.True(t, strings.HasSuffix(filename, ".md"), "filename should end with '.md'")

	// Extract timestamp part and validate format
	// Format: progress-20260115-143022.md
	timestampPart := strings.TrimPrefix(filename, "progress-")
	timestampPart = strings.TrimSuffix(timestampPart, ".md")
	_, err = time.Parse("20060102-150405", timestampPart)
	assert.NoError(t, err, "timestamp in filename should be valid")
}

func TestProgressArchive_Archive_CreatesArchiveDir(t *testing.T) {
	tmpDir := t.TempDir()
	progressPath := filepath.Join(tmpDir, "progress.md")
	archiveDir := filepath.Join(tmpDir, "nested", "archive")

	err := os.WriteFile(progressPath, []byte("test content"), 0644)
	require.NoError(t, err)

	// Archive directory doesn't exist yet
	_, err = os.Stat(archiveDir)
	assert.True(t, os.IsNotExist(err))

	archive := NewProgressArchive(archiveDir)
	_, err = archive.Archive(progressPath)
	require.NoError(t, err)

	// Archive directory should be created
	assert.DirExists(t, archiveDir)
}

func TestProgressArchive_Archive_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	progressPath := filepath.Join(tmpDir, "nonexistent.md")
	archiveDir := filepath.Join(tmpDir, "archive")

	archive := NewProgressArchive(archiveDir)
	_, err := archive.Archive(progressPath)

	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err), "should return not exist error")
}

func TestProgressArchive_Archive_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	progressPath := filepath.Join(tmpDir, "progress.md")
	archiveDir := filepath.Join(tmpDir, "archive")

	// Create empty file
	err := os.WriteFile(progressPath, []byte(""), 0644)
	require.NoError(t, err)

	archive := NewProgressArchive(archiveDir)
	archivedPath, err := archive.Archive(progressPath)
	require.NoError(t, err)

	// Empty file should still be archived
	assert.FileExists(t, archivedPath)
	content, err := os.ReadFile(archivedPath)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestProgressArchive_Archive_MultipleArchives(t *testing.T) {
	tmpDir := t.TempDir()
	progressPath := filepath.Join(tmpDir, "progress.md")
	archiveDir := filepath.Join(tmpDir, "archive")

	archive := NewProgressArchive(archiveDir)

	// First archive
	err := os.WriteFile(progressPath, []byte("first content"), 0644)
	require.NoError(t, err)
	path1, err := archive.Archive(progressPath)
	require.NoError(t, err)

	// Second archive (recreate progress file) - may happen within same second
	err = os.WriteFile(progressPath, []byte("second content"), 0644)
	require.NoError(t, err)
	path2, err := archive.Archive(progressPath)
	require.NoError(t, err)

	// Both files should exist with different names (collision handling adds suffix)
	assert.FileExists(t, path1)
	assert.FileExists(t, path2)
	assert.NotEqual(t, path1, path2)

	// Verify contents
	content1, _ := os.ReadFile(path1)
	content2, _ := os.ReadFile(path2)
	assert.Equal(t, "first content", string(content1))
	assert.Equal(t, "second content", string(content2))
}

func TestProgressArchive_ArchiveDir(t *testing.T) {
	archiveDir := "/custom/archive/path"
	archive := NewProgressArchive(archiveDir)

	assert.Equal(t, archiveDir, archive.ArchiveDir())
}

func TestProgressArchive_ListArchives(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "archive")

	// Create archive directory with some files
	err := os.MkdirAll(archiveDir, 0755)
	require.NoError(t, err)

	// Create archive files
	file1 := filepath.Join(archiveDir, "progress-20260115-100000.md")
	file2 := filepath.Join(archiveDir, "progress-20260116-100000.md")
	file3 := filepath.Join(archiveDir, "other-file.txt") // Should be excluded

	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file3, []byte("other"), 0644)
	require.NoError(t, err)

	archive := NewProgressArchive(archiveDir)
	archives, err := archive.ListArchives()
	require.NoError(t, err)

	// Should only include progress-*.md files, sorted newest first
	assert.Len(t, archives, 2)
	assert.Contains(t, archives, file1)
	assert.Contains(t, archives, file2)
}

func TestProgressArchive_ListArchives_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "archive")

	err := os.MkdirAll(archiveDir, 0755)
	require.NoError(t, err)

	archive := NewProgressArchive(archiveDir)
	archives, err := archive.ListArchives()
	require.NoError(t, err)

	assert.Empty(t, archives)
}

func TestProgressArchive_ListArchives_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "nonexistent")

	archive := NewProgressArchive(archiveDir)
	archives, err := archive.ListArchives()
	require.NoError(t, err)

	// Should return empty list, not error
	assert.Empty(t, archives)
}

func TestGenerateArchiveFilename(t *testing.T) {
	now := time.Date(2026, 1, 15, 14, 30, 22, 0, time.UTC)
	filename := generateArchiveFilename(now)

	assert.Equal(t, "progress-20260115-143022.md", filename)
}

func TestGenerateArchiveFilename_DifferentTimes(t *testing.T) {
	tests := []struct {
		time     time.Time
		expected string
	}{
		{
			time:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "progress-20260101-000000.md",
		},
		{
			time:     time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "progress-20261231-235959.md",
		},
		{
			time:     time.Date(2025, 6, 15, 12, 30, 45, 0, time.UTC),
			expected: "progress-20250615-123045.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			filename := generateArchiveFilename(tc.time)
			assert.Equal(t, tc.expected, filename)
		})
	}
}
