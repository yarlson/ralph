// Package internal provides shared utilities for Ralph CLI commands.
package internal

import (
	"regexp"
	"strings"
)

// FileType represents the detected type of an input file.
type FileType int

// File type constants.
const (
	FileTypeUnknown FileType = iota
	FileTypePRD
	FileTypeTasks
)

// String returns the string representation of the FileType.
func (ft FileType) String() string {
	switch ft {
	case FileTypePRD:
		return "prd"
	case FileTypeTasks:
		return "tasks"
	default:
		return "unknown"
	}
}

// prdPatterns are regex patterns that indicate PRD/spec content.
var prdPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)##\s*objectives`),
	regexp.MustCompile(`(?i)##\s*requirements`),
	regexp.MustCompile(`(?i)##\s*user\s+stories`),
	regexp.MustCompile(`(?i)##\s*acceptance\s+criteria`),
	regexp.MustCompile(`(?i)##\s*overview`),
}

// taskPatterns are patterns that indicate task YAML content.
var taskPatterns = []string{
	"id:",
	"tasks:",
	"status:",
	"depends_on:",
}

// DetectFileType analyzes content and returns the detected file type.
// It checks for PRD markers first, then task YAML patterns.
func DetectFileType(content string) FileType {
	// Check for PRD patterns (markdown sections)
	for _, pattern := range prdPatterns {
		if pattern.MatchString(content) {
			return FileTypePRD
		}
	}

	// Check for task YAML patterns
	for _, pattern := range taskPatterns {
		if strings.Contains(content, pattern) {
			return FileTypeTasks
		}
	}

	return FileTypeUnknown
}
