package loop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "feature",
			expected: "feature",
		},
		{
			name:     "uppercase converted",
			input:    "Feature Branch",
			expected: "feature-branch",
		},
		{
			name:     "spaces to hyphens",
			input:    "my new feature",
			expected: "my-new-feature",
		},
		{
			name:     "underscores to hyphens",
			input:    "my_new_feature",
			expected: "my-new-feature",
		},
		{
			name:     "special characters removed",
			input:    "feature@#$%name!",
			expected: "featurename",
		},
		{
			name:     "multiple consecutive hyphens collapsed",
			input:    "feature---name",
			expected: "feature-name",
		},
		{
			name:     "leading hyphens trimmed",
			input:    "---feature",
			expected: "feature",
		},
		{
			name:     "trailing hyphens trimmed",
			input:    "feature---",
			expected: "feature",
		},
		{
			name:     "mixed special characters and spaces",
			input:    "Feature: Name (Test)",
			expected: "feature-name-test",
		},
		{
			name:     "empty string returns unknown",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "only special characters returns unknown",
			input:    "@#$%^&*()",
			expected: "unknown",
		},
		{
			name:     "numbers preserved",
			input:    "feature-v2.0",
			expected: "feature-v20",
		},
		{
			name:     "real task title example",
			input:    "Ralph PRD Alignment and Completion",
			expected: "ralph-prd-alignment-and-completion",
		},
		{
			name:     "CLI and Configuration Foundation",
			input:    "CLI and Configuration Foundation",
			expected: "cli-and-configuration-foundation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slugify(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
