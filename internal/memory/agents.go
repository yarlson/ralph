package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxAgentsBytes is the maximum size per AGENTS.md file to include in prompts.
	MaxAgentsBytes = 10 * 1024 // 10KB per file
)

// FindAgentsMd searches for all AGENTS.md files in the directory tree starting from root.
// It returns the absolute paths to all found AGENTS.md files.
func FindAgentsMd(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// If we can't read a directory, skip it
			return nil
		}

		// Skip hidden directories (like .git, .ralph)
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != root {
			return filepath.SkipDir
		}

		// Check if this is an AGENTS.md file
		if !d.IsDir() && d.Name() == "AGENTS.md" {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil // Skip files we can't get absolute path for
			}
			files = append(files, absPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory tree: %w", err)
	}

	return files, nil
}

// ReadAgentsMd finds and reads all AGENTS.md files in the directory tree,
// returning their concatenated content formatted for inclusion in prompts.
// Each file's content is truncated to MaxAgentsBytes and includes the file path.
// Returns an empty string if no AGENTS.md files are found.
func ReadAgentsMd(root string) (string, error) {
	files, err := FindAgentsMd(root)
	if err != nil {
		return "", fmt.Errorf("finding AGENTS.md files: %w", err)
	}

	if len(files) == 0 {
		return "", nil
	}

	var sb strings.Builder

	for i, filePath := range files {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			// If we can't read a file, skip it silently
			continue
		}

		// Truncate if needed
		contentStr := string(content)
		if len(contentStr) > MaxAgentsBytes {
			contentStr = contentStr[:MaxAgentsBytes] + "\n\n... [truncated]"
		}

		// Format with file path header
		_, _ = fmt.Fprintf(&sb, "### From: %s\n\n%s", filePath, contentStr)
	}

	return sb.String(), nil
}
