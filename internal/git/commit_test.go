package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       CommitType
		expected string
	}{
		{"feat", CommitTypeFeat, "feat"},
		{"fix", CommitTypeFix, "fix"},
		{"chore", CommitTypeChore, "chore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ct.String())
		})
	}
}

func TestInferCommitType(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected CommitType
	}{
		// feat detection
		{"feat from add keyword", "Add new feature", CommitTypeFeat},
		{"feat from implement keyword", "Implement user authentication", CommitTypeFeat},
		{"feat from create keyword", "Create task store", CommitTypeFeat},
		{"feat from new keyword", "New verification pipeline", CommitTypeFeat},
		{"feat case insensitive", "ADD new feature", CommitTypeFeat},

		// fix detection
		{"fix from fix keyword", "Fix bug in selector", CommitTypeFix},
		{"fix from repair keyword", "Repair broken test", CommitTypeFix},
		{"fix from resolve keyword", "Resolve issue with parsing", CommitTypeFix},
		{"fix from correct keyword", "Correct validation logic", CommitTypeFix},
		{"fix case insensitive", "FIX broken test", CommitTypeFix},

		// chore detection (default and explicit)
		{"chore from update keyword", "Update dependencies", CommitTypeChore},
		{"chore from refactor keyword", "Refactor selector code", CommitTypeChore},
		{"chore from clean keyword", "Clean up unused code", CommitTypeChore},
		{"chore from remove keyword", "Remove deprecated method", CommitTypeChore},
		{"chore from rename keyword", "Rename variable for clarity", CommitTypeChore},
		{"chore from move keyword", "Move file to new location", CommitTypeChore},

		// default to chore when no keyword matches
		{"default to chore", "Task store model definition", CommitTypeChore},
		{"default to chore for generic", "Some random task title", CommitTypeChore},
		{"empty string defaults to chore", "", CommitTypeChore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InferCommitType(tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCommitMessage(t *testing.T) {
	tests := []struct {
		name        string
		taskTitle   string
		iterationID string
		expected    string
	}{
		{
			name:        "feat commit",
			taskTitle:   "Add user authentication",
			iterationID: "iter-001",
			expected:    "feat: Add user authentication\n\nRalph iteration: iter-001",
		},
		{
			name:        "fix commit",
			taskTitle:   "Fix validation bug",
			iterationID: "iter-002",
			expected:    "fix: Fix validation bug\n\nRalph iteration: iter-002",
		},
		{
			name:        "chore commit",
			taskTitle:   "Update dependencies",
			iterationID: "iter-003",
			expected:    "chore: Update dependencies\n\nRalph iteration: iter-003",
		},
		{
			name:        "default chore commit",
			taskTitle:   "Task store model",
			iterationID: "iter-004",
			expected:    "chore: Task store model\n\nRalph iteration: iter-004",
		},
		{
			name:        "empty iteration ID",
			taskTitle:   "Add feature",
			iterationID: "",
			expected:    "feat: Add feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCommitMessage(tt.taskTitle, tt.iterationID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCommitMessageWithType(t *testing.T) {
	tests := []struct {
		name        string
		commitType  CommitType
		taskTitle   string
		iterationID string
		expected    string
	}{
		{
			name:        "explicit feat",
			commitType:  CommitTypeFeat,
			taskTitle:   "Something",
			iterationID: "iter-001",
			expected:    "feat: Something\n\nRalph iteration: iter-001",
		},
		{
			name:        "explicit fix",
			commitType:  CommitTypeFix,
			taskTitle:   "Something",
			iterationID: "iter-002",
			expected:    "fix: Something\n\nRalph iteration: iter-002",
		},
		{
			name:        "explicit chore",
			commitType:  CommitTypeChore,
			taskTitle:   "Something",
			iterationID: "iter-003",
			expected:    "chore: Something\n\nRalph iteration: iter-003",
		},
		{
			name:        "no iteration metadata",
			commitType:  CommitTypeFeat,
			taskTitle:   "New feature",
			iterationID: "",
			expected:    "feat: New feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCommitMessageWithType(tt.commitType, tt.taskTitle, tt.iterationID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseConventionalCommit(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedType    CommitType
		expectedSubject string
		expectedBody    string
	}{
		{
			name:            "feat with body",
			message:         "feat: Add feature\n\nRalph iteration: iter-001",
			expectedType:    CommitTypeFeat,
			expectedSubject: "Add feature",
			expectedBody:    "Ralph iteration: iter-001",
		},
		{
			name:            "fix without body",
			message:         "fix: Fix bug",
			expectedType:    CommitTypeFix,
			expectedSubject: "Fix bug",
			expectedBody:    "",
		},
		{
			name:            "chore with body",
			message:         "chore: Update deps\n\nSome body text",
			expectedType:    CommitTypeChore,
			expectedSubject: "Update deps",
			expectedBody:    "Some body text",
		},
		{
			name:            "non-conventional message",
			message:         "Just a regular commit message",
			expectedType:    "",
			expectedSubject: "",
			expectedBody:    "",
		},
		{
			name:            "empty message",
			message:         "",
			expectedType:    "",
			expectedSubject: "",
			expectedBody:    "",
		},
		{
			name:            "multiline body",
			message:         "feat: Add feature\n\nLine 1\nLine 2\nLine 3",
			expectedType:    CommitTypeFeat,
			expectedSubject: "Add feature",
			expectedBody:    "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, subject, body := ParseConventionalCommit(tt.message)
			assert.Equal(t, tt.expectedType, ct)
			assert.Equal(t, tt.expectedSubject, subject)
			assert.Equal(t, tt.expectedBody, body)
		})
	}
}

func TestValidateCommitMessage(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		expectErr bool
	}{
		{"valid feat message", "feat: Add feature", false},
		{"valid fix message", "fix: Fix bug", false},
		{"valid chore message", "chore: Update deps", false},
		{"valid with body", "feat: Add feature\n\nBody text", false},
		{"empty message", "", true},
		{"no colon", "feat Add feature", true},
		{"no type", ": Add feature", true},
		{"empty subject", "feat:", true},
		{"whitespace only subject", "feat:   ", true},
		{"unknown type", "unknown: Something", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommitMessage(tt.message)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
