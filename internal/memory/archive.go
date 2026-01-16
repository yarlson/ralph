package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProgressArchive manages archiving of progress files.
type ProgressArchive struct {
	archiveDir string
}

// NewProgressArchive creates a new ProgressArchive manager for the given directory.
func NewProgressArchive(archiveDir string) *ProgressArchive {
	return &ProgressArchive{archiveDir: archiveDir}
}

// Archive moves the progress file to the archive directory with a timestamp in the filename.
// Returns the path to the archived file.
func (a *ProgressArchive) Archive(progressPath string) (string, error) {
	// Check if source file exists
	if _, err := os.Stat(progressPath); err != nil {
		return "", err
	}

	// Create archive directory if it doesn't exist
	if err := os.MkdirAll(a.archiveDir, 0755); err != nil {
		return "", fmt.Errorf("creating archive directory: %w", err)
	}

	// Generate timestamped filename, handling collisions
	archivedPath := a.generateUniqueArchivePath(time.Now())

	// Read the source file content
	content, err := os.ReadFile(progressPath)
	if err != nil {
		return "", fmt.Errorf("reading progress file: %w", err)
	}

	// Write to archive location
	if err := os.WriteFile(archivedPath, content, 0644); err != nil {
		return "", fmt.Errorf("writing archived file: %w", err)
	}

	// Remove the original file
	if err := os.Remove(progressPath); err != nil {
		// Try to clean up the archived file if we can't remove the original
		_ = os.Remove(archivedPath)
		return "", fmt.Errorf("removing original file: %w", err)
	}

	return archivedPath, nil
}

// generateUniqueArchivePath generates a unique archive path, handling collisions
// by appending a counter suffix if needed.
func (a *ProgressArchive) generateUniqueArchivePath(t time.Time) string {
	baseFilename := generateArchiveFilename(t)
	basePath := filepath.Join(a.archiveDir, baseFilename)

	// Check if file exists, if not use base path
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return basePath
	}

	// File exists, try with counter suffix
	ext := filepath.Ext(baseFilename)
	nameWithoutExt := strings.TrimSuffix(baseFilename, ext)

	for i := 1; i < 1000; i++ {
		suffixedPath := filepath.Join(a.archiveDir, fmt.Sprintf("%s-%d%s", nameWithoutExt, i, ext))
		if _, err := os.Stat(suffixedPath); os.IsNotExist(err) {
			return suffixedPath
		}
	}

	// Fallback (should never reach here in practice)
	return basePath
}

// ArchiveDir returns the archive directory path.
func (a *ProgressArchive) ArchiveDir() string {
	return a.archiveDir
}

// ListArchives returns a list of all archived progress files, sorted by filename (newest first).
func (a *ProgressArchive) ListArchives() ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(a.archiveDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(a.archiveDir)
	if err != nil {
		return nil, fmt.Errorf("reading archive directory: %w", err)
	}

	var archives []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only include progress-*.md files
		if strings.HasPrefix(name, "progress-") && strings.HasSuffix(name, ".md") {
			archives = append(archives, filepath.Join(a.archiveDir, name))
		}
	}

	// Sort by filename descending (newest first based on timestamp in name)
	sort.Sort(sort.Reverse(sort.StringSlice(archives)))

	return archives, nil
}

// generateArchiveFilename creates a timestamped filename for the archived progress file.
// Format: progress-YYYYMMDD-HHMMSS.md
func generateArchiveFilename(t time.Time) string {
	return fmt.Sprintf("progress-%s.md", t.Format("20060102-150405"))
}
