// Package memory provides progress and memory file management for Ralph iterations.
package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProgressFile manages the .ralph/progress.md file for tracking iteration history.
type ProgressFile struct {
	path string
}

// IterationEntry represents a single iteration entry in the progress log.
type IterationEntry struct {
	TaskID       string
	TaskTitle    string
	WhatChanged  []string
	FilesTouched []string
	Learnings    []string
	Outcome      string
}

// SizeOptions configures the maximum size limits for the progress file.
type SizeOptions struct {
	MaxBytes            int // Maximum file size in bytes (0 = unlimited)
	MaxRecentIterations int // Minimum number of recent iterations to preserve
}

// NewProgressFile creates a new ProgressFile manager for the given path.
func NewProgressFile(path string) *ProgressFile {
	return &ProgressFile{path: path}
}

// Init creates a new progress file with the standard header.
// If the file already exists, it does nothing.
func (p *ProgressFile) Init(featureName, parentTaskID string) error {
	if p.Exists() {
		return nil
	}

	// Create parent directories if needed
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating progress directory: %w", err)
	}

	header := p.formatHeader(featureName, parentTaskID)
	if err := os.WriteFile(p.path, []byte(header), 0644); err != nil {
		return fmt.Errorf("writing progress file: %w", err)
	}

	return nil
}

// formatHeader creates the standard progress file header.
func (p *ProgressFile) formatHeader(featureName, parentTaskID string) string {
	now := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`# Ralph MVP Progress

**Feature**: %s
**Parent Task**: %s
**Started**: %s

---

## Codebase Patterns

<!-- Add patterns discovered during implementation here -->

---

## Iteration Log

`, featureName, parentTaskID, now)
}

// AppendIteration appends a new iteration entry to the progress file.
func (p *ProgressFile) AppendIteration(entry IterationEntry) error {
	// Create parent directories if needed
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating progress directory: %w", err)
	}

	// Format the entry
	formatted := entry.Format(time.Now())

	// Open file for appending (create if not exists)
	f, err := os.OpenFile(p.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening progress file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(formatted); err != nil {
		return fmt.Errorf("appending to progress file: %w", err)
	}

	return nil
}

// Format formats the iteration entry as markdown.
func (e *IterationEntry) Format(timestamp time.Time) string {
	var sb strings.Builder

	// Header with date, task ID, and title
	_, _ = fmt.Fprintf(&sb, "### %s: %s (%s)\n\n",
		timestamp.Format("2006-01-02"), e.TaskID, e.TaskTitle)

	// What changed (always present)
	sb.WriteString("**What changed:**\n")
	for _, change := range e.WhatChanged {
		_, _ = fmt.Fprintf(&sb, "- %s\n", change)
	}
	sb.WriteString("\n")

	// Files touched (optional)
	if len(e.FilesTouched) > 0 {
		sb.WriteString("**Files touched:**\n")
		for _, file := range e.FilesTouched {
			_, _ = fmt.Fprintf(&sb, "- `%s`\n", file)
		}
		sb.WriteString("\n")
	}

	// Learnings (optional)
	if len(e.Learnings) > 0 {
		sb.WriteString("**Learnings:**\n")
		for _, learning := range e.Learnings {
			_, _ = fmt.Fprintf(&sb, "- %s\n", learning)
		}
		sb.WriteString("\n")
	}

	// Outcome
	_, _ = fmt.Fprintf(&sb, "**Outcome**: %s\n\n", e.Outcome)

	return sb.String()
}

// GetCodebasePatterns extracts the Codebase Patterns section from the progress file.
func (p *ProgressFile) GetCodebasePatterns() (string, error) {
	if !p.Exists() {
		return "", nil
	}

	content, err := os.ReadFile(p.path)
	if err != nil {
		return "", fmt.Errorf("reading progress file: %w", err)
	}

	return extractSection(string(content), "## Codebase Patterns", "---")
}

// extractSection extracts content between a start marker and end marker.
func extractSection(content, startMarker, endMarker string) (string, error) {
	_, afterStart, found := strings.Cut(content, startMarker)
	if !found {
		return "", nil
	}

	// Find the next occurrence of end marker
	section, _, found := strings.Cut(afterStart, endMarker)
	if !found {
		// No end marker, take rest of content
		return strings.TrimSpace(afterStart), nil
	}

	return strings.TrimSpace(section), nil
}

// UpdateCodebasePatterns replaces the Codebase Patterns section in the progress file.
func (p *ProgressFile) UpdateCodebasePatterns(patterns string) error {
	if !p.Exists() {
		return errors.New("progress file does not exist")
	}

	content, err := os.ReadFile(p.path)
	if err != nil {
		return fmt.Errorf("reading progress file: %w", err)
	}

	updated, err := replaceSection(string(content), "## Codebase Patterns", "---", patterns)
	if err != nil {
		return fmt.Errorf("replacing patterns section: %w", err)
	}

	if err := os.WriteFile(p.path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing progress file: %w", err)
	}

	return nil
}

// replaceSection replaces content between start and end markers with new content.
func replaceSection(content, startMarker, endMarker, newContent string) (string, error) {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return content, nil
	}

	// Find where the section content starts (after the marker line)
	afterStart := content[startIdx+len(startMarker):]
	endIdx := strings.Index(afterStart, endMarker)
	if endIdx == -1 {
		return content, nil
	}

	// Build the new content
	before := content[:startIdx+len(startMarker)]
	after := afterStart[endIdx:]

	return before + "\n\n" + newContent + "\n\n" + after, nil
}

// EnforceMaxSize prunes old iteration entries if the file exceeds the configured limits.
// It preserves the header and Codebase Patterns section, and keeps at least MaxRecentIterations
// of the most recent entries. Returns true if pruning was performed.
func (p *ProgressFile) EnforceMaxSize(opts SizeOptions) (bool, error) {
	if !p.Exists() {
		return false, nil
	}

	content, err := os.ReadFile(p.path)
	if err != nil {
		return false, fmt.Errorf("reading progress file: %w", err)
	}

	// Check if we need to prune based on byte size
	if opts.MaxBytes == 0 || len(content) <= opts.MaxBytes {
		return false, nil
	}

	// Parse the file structure
	text := string(content)
	lines := strings.Split(text, "\n")

	// Find the iteration log section
	iterationLogIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "## Iteration Log") {
			iterationLogIdx = i
			break
		}
	}

	if iterationLogIdx == -1 {
		// No iteration log section, can't prune
		return false, nil
	}

	// Preserve header (everything before and including "## Iteration Log" + blank line)
	// This includes the file header and the Codebase Patterns section
	headerEndIdx := iterationLogIdx + 1
	if headerEndIdx < len(lines) && lines[headerEndIdx] == "" {
		headerEndIdx++ // Include the blank line after "## Iteration Log"
	}
	headerLines := lines[:headerEndIdx]

	// Get iteration entries
	iterationLines := lines[headerEndIdx:]
	entries := splitIntoEntries(iterationLines)
	if len(entries) == 0 {
		return false, nil
	}

	// Always keep at least MaxRecentIterations entries
	minKeepEntries := opts.MaxRecentIterations
	if minKeepEntries <= 0 {
		minKeepEntries = 1 // Always keep at least one entry
	}
	if len(entries) <= minKeepEntries {
		return false, nil // Not enough entries to prune
	}

	// Calculate how many entries to keep to stay under size limit
	// Build header first to know its size
	var headerBuilder strings.Builder
	for _, line := range headerLines {
		headerBuilder.WriteString(line)
		headerBuilder.WriteString("\n")
	}
	headerSize := headerBuilder.Len()

	// Start with minimum required iterations and see if we're under limit
	keepFrom := len(entries) - minKeepEntries
	for keepFrom > 0 {
		testSize := headerSize
		for i := keepFrom; i < len(entries); i++ {
			testSize += len(entries[i])
			if i < len(entries)-1 {
				testSize++ // newline between entries
			}
		}
		if testSize <= opts.MaxBytes {
			break // Found a good pruning point
		}
		// Still too large, try removing one more old entry
		keepFrom++
	}

	if keepFrom == 0 {
		// Even with all entries we're over limit, but we must keep minimum recent
		keepFrom = len(entries) - minKeepEntries
	}

	if keepFrom == 0 {
		return false, nil // No pruning needed
	}

	// Build pruned content
	var sb strings.Builder
	sb.WriteString(headerBuilder.String())

	// Add pruning note if we removed entries
	if keepFrom > 0 {
		sb.WriteString("<!-- Older entries pruned to maintain size limit -->\n\n")
	}

	for i := keepFrom; i < len(entries); i++ {
		sb.WriteString(entries[i])
		if i < len(entries)-1 {
			sb.WriteString("\n")
		}
	}

	if err := os.WriteFile(p.path, []byte(sb.String()), 0644); err != nil {
		return false, fmt.Errorf("writing pruned progress file: %w", err)
	}

	return true, nil
}

// splitIntoEntries splits the iteration log into individual entries.
func splitIntoEntries(lines []string) []string {
	var entries []string
	var currentEntry strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") && currentEntry.Len() > 0 {
			entries = append(entries, currentEntry.String())
			currentEntry.Reset()
		}
		if currentEntry.Len() > 0 || strings.HasPrefix(line, "### ") || line != "" {
			currentEntry.WriteString(line)
			currentEntry.WriteString("\n")
		}
	}

	if currentEntry.Len() > 0 {
		entries = append(entries, strings.TrimSuffix(currentEntry.String(), "\n"))
	}

	return entries
}

// Exists returns true if the progress file exists.
func (p *ProgressFile) Exists() bool {
	_, err := os.Stat(p.path)
	return err == nil
}

// Path returns the file path of the progress file.
func (p *ProgressFile) Path() string {
	return p.path
}
