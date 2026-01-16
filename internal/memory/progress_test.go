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

func TestNewProgressFile(t *testing.T) {
	t.Run("creates progress file with header", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task-123")
		require.NoError(t, err)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)

		// Check header
		assert.Contains(t, string(content), "# Ralph MVP Progress")
		assert.Contains(t, string(content), "**Feature**: Test Feature")
		assert.Contains(t, string(content), "**Parent Task**: test-task-123")
		assert.Contains(t, string(content), "**Started**:")
		assert.Contains(t, string(content), "## Codebase Patterns")
		assert.Contains(t, string(content), "## Iteration Log")
	})

	t.Run("creates parent directories if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "subdir", "nested", "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		_, err = os.Stat(progressPath)
		require.NoError(t, err)
	})

	t.Run("does not overwrite existing progress file", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		existingContent := "# Existing Progress\nSome content here"
		err := os.WriteFile(progressPath, []byte(existingContent), 0644)
		require.NoError(t, err)

		pf := NewProgressFile(progressPath)
		err = pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		assert.Equal(t, existingContent, string(content))
	})
}

func TestProgressFile_AppendIteration(t *testing.T) {
	t.Run("appends iteration entry to progress file", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		entry := IterationEntry{
			TaskID:     "task-123",
			TaskTitle:  "Implement feature X",
			WhatChanged: []string{
				"Added new function foo()",
				"Updated bar() to handle edge case",
			},
			FilesTouched: []string{
				"internal/foo/foo.go",
				"internal/bar/bar.go",
			},
			Learnings: []string{
				"Use context for cancellation",
			},
			Outcome: "Success",
		}

		err = pf.AppendIteration(entry)
		require.NoError(t, err)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.Contains(t, contentStr, "### ")
		assert.Contains(t, contentStr, "task-123 (Implement feature X)")
		assert.Contains(t, contentStr, "**What changed:**")
		assert.Contains(t, contentStr, "- Added new function foo()")
		assert.Contains(t, contentStr, "- Updated bar() to handle edge case")
		assert.Contains(t, contentStr, "**Files touched:**")
		assert.Contains(t, contentStr, "- `internal/foo/foo.go`")
		assert.Contains(t, contentStr, "**Learnings:**")
		assert.Contains(t, contentStr, "- Use context for cancellation")
		assert.Contains(t, contentStr, "**Outcome**: Success")
	})

	t.Run("appends multiple iterations", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		entry1 := IterationEntry{
			TaskID:      "task-1",
			TaskTitle:   "First task",
			WhatChanged: []string{"Change 1"},
			Outcome:     "Success",
		}
		entry2 := IterationEntry{
			TaskID:      "task-2",
			TaskTitle:   "Second task",
			WhatChanged: []string{"Change 2"},
			Outcome:     "Success",
		}

		err = pf.AppendIteration(entry1)
		require.NoError(t, err)
		err = pf.AppendIteration(entry2)
		require.NoError(t, err)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.Contains(t, contentStr, "task-1 (First task)")
		assert.Contains(t, contentStr, "task-2 (Second task)")
	})

	t.Run("creates file if it does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		entry := IterationEntry{
			TaskID:      "task-1",
			TaskTitle:   "First task",
			WhatChanged: []string{"Change 1"},
			Outcome:     "Success",
		}

		err := pf.AppendIteration(entry)
		require.NoError(t, err)

		_, err = os.Stat(progressPath)
		require.NoError(t, err)
	})

	t.Run("handles empty optional fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		entry := IterationEntry{
			TaskID:      "task-1",
			TaskTitle:   "Minimal task",
			WhatChanged: []string{"Did something"},
			Outcome:     "Success",
			// FilesTouched and Learnings are empty
		}

		err = pf.AppendIteration(entry)
		require.NoError(t, err)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		contentStr := string(content)

		// Should not contain empty sections
		assert.NotContains(t, contentStr, "**Files touched:**\n\n")
		assert.NotContains(t, contentStr, "**Learnings:**\n\n")
	})
}

func TestProgressFile_GetCodebasePatterns(t *testing.T) {
	t.Run("extracts patterns section from progress file", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		content := `# Ralph MVP Progress

**Feature**: Test Feature
**Parent Task**: test-task
**Started**: 2026-01-16

---

## Codebase Patterns

- **Config loading**: Use Viper with SetDefault() for all fields
- **Testing**: Use t.TempDir() for isolated test fixtures

---

## Iteration Log

### 2026-01-16: task-1 (First task)
Some iteration content here
`
		err := os.WriteFile(progressPath, []byte(content), 0644)
		require.NoError(t, err)

		pf := NewProgressFile(progressPath)
		patterns, err := pf.GetCodebasePatterns()
		require.NoError(t, err)

		assert.Contains(t, patterns, "- **Config loading**: Use Viper with SetDefault() for all fields")
		assert.Contains(t, patterns, "- **Testing**: Use t.TempDir() for isolated test fixtures")
		assert.NotContains(t, patterns, "## Iteration Log")
		assert.NotContains(t, patterns, "Some iteration content")
	})

	t.Run("returns empty string if no patterns section", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		content := `# Ralph MVP Progress

Some other content without patterns section
`
		err := os.WriteFile(progressPath, []byte(content), 0644)
		require.NoError(t, err)

		pf := NewProgressFile(progressPath)
		patterns, err := pf.GetCodebasePatterns()
		require.NoError(t, err)
		assert.Empty(t, patterns)
	})

	t.Run("returns empty string if file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "nonexistent.md")

		pf := NewProgressFile(progressPath)
		patterns, err := pf.GetCodebasePatterns()
		require.NoError(t, err)
		assert.Empty(t, patterns)
	})
}

func TestProgressFile_UpdateCodebasePatterns(t *testing.T) {
	t.Run("updates patterns section in progress file", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		newPatterns := `- **New pattern**: Some new guidance
- **Another pattern**: More guidance`

		err = pf.UpdateCodebasePatterns(newPatterns)
		require.NoError(t, err)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		contentStr := string(content)

		assert.Contains(t, contentStr, "- **New pattern**: Some new guidance")
		assert.Contains(t, contentStr, "- **Another pattern**: More guidance")
	})

	t.Run("preserves other sections when updating patterns", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		content := `# Ralph MVP Progress

**Feature**: Test Feature
**Parent Task**: test-task
**Started**: 2026-01-16

---

## Codebase Patterns

- **Old pattern**: Old guidance

---

## Iteration Log

### 2026-01-16: task-1 (First task)
Some iteration content here
`
		err := os.WriteFile(progressPath, []byte(content), 0644)
		require.NoError(t, err)

		pf := NewProgressFile(progressPath)
		err = pf.UpdateCodebasePatterns("- **New pattern**: New guidance")
		require.NoError(t, err)

		updatedContent, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		contentStr := string(updatedContent)

		// Patterns should be updated
		assert.Contains(t, contentStr, "- **New pattern**: New guidance")
		assert.NotContains(t, contentStr, "- **Old pattern**: Old guidance")

		// Other sections preserved
		assert.Contains(t, contentStr, "# Ralph MVP Progress")
		assert.Contains(t, contentStr, "**Feature**: Test Feature")
		assert.Contains(t, contentStr, "## Iteration Log")
		assert.Contains(t, contentStr, "### 2026-01-16: task-1 (First task)")
	})
}

func TestProgressFile_EnforceMaxSize(t *testing.T) {
	t.Run("prunes old entries when over line limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		// Add many iterations to exceed size limit
		for i := 0; i < 20; i++ {
			entry := IterationEntry{
				TaskID:      "task-" + string(rune('A'+i)),
				TaskTitle:   "Task " + string(rune('A'+i)),
				WhatChanged: []string{"Change " + string(rune('A'+i)), "More changes here to increase size"},
				Learnings:   []string{"Learning " + string(rune('A'+i)), "Another learning"},
				Outcome:     "Success",
			}
			err := pf.AppendIteration(entry)
			require.NoError(t, err)
		}

		// Set a low line limit
		opts := SizeOptions{MaxLines: 50}
		pruned, err := pf.EnforceMaxSize(opts)
		require.NoError(t, err)
		assert.True(t, pruned)

		content, err := os.ReadFile(progressPath)
		require.NoError(t, err)

		lines := strings.Split(string(content), "\n")
		assert.LessOrEqual(t, len(lines), opts.MaxLines+5) // Allow some tolerance

		// Header should be preserved
		assert.Contains(t, string(content), "# Ralph MVP Progress")
		assert.Contains(t, string(content), "## Codebase Patterns")
	})

	t.Run("preserves header and patterns when pruning", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		content := `# Ralph MVP Progress

**Feature**: Test Feature
**Parent Task**: test-task
**Started**: 2026-01-16

---

## Codebase Patterns

- **Important pattern**: Must be preserved

---

## Iteration Log

### Old Entry 1
Content 1

### Old Entry 2
Content 2

### Old Entry 3
Content 3
`
		err := os.WriteFile(progressPath, []byte(content), 0644)
		require.NoError(t, err)

		pf := NewProgressFile(progressPath)
		opts := SizeOptions{MaxLines: 20}
		_, err = pf.EnforceMaxSize(opts)
		require.NoError(t, err)

		updated, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		contentStr := string(updated)

		// Header and patterns must be preserved
		assert.Contains(t, contentStr, "# Ralph MVP Progress")
		assert.Contains(t, contentStr, "**Feature**: Test Feature")
		assert.Contains(t, contentStr, "## Codebase Patterns")
		assert.Contains(t, contentStr, "- **Important pattern**: Must be preserved")
	})

	t.Run("does nothing if under limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		pf := NewProgressFile(progressPath)
		err := pf.Init("Test Feature", "test-task")
		require.NoError(t, err)

		beforeContent, err := os.ReadFile(progressPath)
		require.NoError(t, err)

		opts := SizeOptions{MaxLines: 1000}
		pruned, err := pf.EnforceMaxSize(opts)
		require.NoError(t, err)
		assert.False(t, pruned)

		afterContent, err := os.ReadFile(progressPath)
		require.NoError(t, err)
		assert.Equal(t, string(beforeContent), string(afterContent))
	})
}

func TestProgressFile_Exists(t *testing.T) {
	t.Run("returns true if file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "progress.md")

		err := os.WriteFile(progressPath, []byte("content"), 0644)
		require.NoError(t, err)

		pf := NewProgressFile(progressPath)
		assert.True(t, pf.Exists())
	})

	t.Run("returns false if file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		progressPath := filepath.Join(tmpDir, "nonexistent.md")

		pf := NewProgressFile(progressPath)
		assert.False(t, pf.Exists())
	})
}

func TestIterationEntry_Format(t *testing.T) {
	t.Run("formats entry with all fields", func(t *testing.T) {
		entry := IterationEntry{
			TaskID:       "test-task-123",
			TaskTitle:    "Implement test feature",
			WhatChanged:  []string{"Added foo", "Updated bar"},
			FilesTouched: []string{"file1.go", "file2.go"},
			Learnings:    []string{"Learning 1", "Learning 2"},
			Outcome:      "Success",
		}

		formatted := entry.Format(time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC))

		assert.Contains(t, formatted, "### 2026-01-16: test-task-123 (Implement test feature)")
		assert.Contains(t, formatted, "**What changed:**")
		assert.Contains(t, formatted, "- Added foo")
		assert.Contains(t, formatted, "- Updated bar")
		assert.Contains(t, formatted, "**Files touched:**")
		assert.Contains(t, formatted, "- `file1.go`")
		assert.Contains(t, formatted, "- `file2.go`")
		assert.Contains(t, formatted, "**Learnings:**")
		assert.Contains(t, formatted, "- Learning 1")
		assert.Contains(t, formatted, "- Learning 2")
		assert.Contains(t, formatted, "**Outcome**: Success")
	})

	t.Run("omits empty optional sections", func(t *testing.T) {
		entry := IterationEntry{
			TaskID:      "test-task",
			TaskTitle:   "Simple task",
			WhatChanged: []string{"Changed something"},
			Outcome:     "Success",
		}

		formatted := entry.Format(time.Now())

		assert.Contains(t, formatted, "**What changed:**")
		assert.NotContains(t, formatted, "**Files touched:**")
		assert.NotContains(t, formatted, "**Learnings:**")
	})
}
