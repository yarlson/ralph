package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileType_String(t *testing.T) {
	tests := []struct {
		name     string
		ft       FileType
		expected string
	}{
		{"unknown", FileTypeUnknown, "unknown"},
		{"prd", FileTypePRD, "prd"},
		{"tasks", FileTypeTasks, "tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ft.String())
		})
	}
}

func TestDetectFileType_PRDPatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected FileType
	}{
		{
			name:     "PRD with objectives section",
			content:  "# Product Spec\n\n## Objectives\n\nBuild a great product.",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with requirements section",
			content:  "# Spec\n\n## Requirements\n\n- Feature A\n- Feature B",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with user stories section",
			content:  "# Document\n\n## User Stories\n\nAs a user I want...",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with acceptance criteria section",
			content:  "# Feature\n\n## Acceptance Criteria\n\n- Must work",
			expected: FileTypePRD,
		},
		{
			name:     "PRD with overview section",
			content:  "# Product\n\n## Overview\n\nThis is the overview.",
			expected: FileTypePRD,
		},
		{
			name:     "PRD case insensitive",
			content:  "# doc\n\n## OBJECTIVES\n\nSome text",
			expected: FileTypePRD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectFileType_TaskPatterns(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected FileType
	}{
		{
			name:     "YAML task with id field",
			content:  "id: task-001\ntitle: Do something\nstatus: open",
			expected: FileTypeTasks,
		},
		{
			name:     "YAML task with tasks array",
			content:  "tasks:\n  - id: task-001\n    title: Task one",
			expected: FileTypeTasks,
		},
		{
			name:     "YAML with status field",
			content:  "title: Some task\nstatus: in_progress\ndescription: Do stuff",
			expected: FileTypeTasks,
		},
		{
			name:     "YAML with depends_on field",
			content:  "id: task-002\ntitle: Task two\ndepends_on:\n  - task-001",
			expected: FileTypeTasks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectFileType_Unknown(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty content", ""},
		{"random text", "Hello world, this is just some text."},
		{"code snippet", "func main() { fmt.Println(\"hello\") }"},
		{"markdown without PRD markers", "# Title\n\nSome paragraph text."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.content)
			assert.Equal(t, FileTypeUnknown, result)
		})
	}
}

func TestFileType_Constants(t *testing.T) {
	// Verify constants have expected values
	assert.Equal(t, FileType(0), FileTypeUnknown)
	assert.Equal(t, FileType(1), FileTypePRD)
	assert.Equal(t, FileType(2), FileTypeTasks)
}
