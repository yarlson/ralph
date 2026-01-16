package git

import (
	"fmt"
	"strings"
)

// CommitType represents the type prefix for conventional commits.
type CommitType string

// Supported commit types following conventional commits specification.
const (
	// CommitTypeFeat indicates a new feature.
	CommitTypeFeat CommitType = "feat"

	// CommitTypeFix indicates a bug fix.
	CommitTypeFix CommitType = "fix"

	// CommitTypeChore indicates maintenance or other changes.
	CommitTypeChore CommitType = "chore"
)

// validCommitTypes contains all supported commit types for validation.
var validCommitTypes = map[CommitType]bool{
	CommitTypeFeat:  true,
	CommitTypeFix:   true,
	CommitTypeChore: true,
}

// String returns the string representation of the commit type.
func (ct CommitType) String() string {
	return string(ct)
}

// IsValid returns true if the commit type is a supported value.
func (ct CommitType) IsValid() bool {
	return validCommitTypes[ct]
}

// featKeywords are keywords that indicate a feat commit type.
var featKeywords = []string{
	"add",
	"implement",
	"create",
	"new",
}

// fixKeywords are keywords that indicate a fix commit type.
var fixKeywords = []string{
	"fix",
	"repair",
	"resolve",
	"correct",
}

// choreKeywords are keywords that indicate a chore commit type.
var choreKeywords = []string{
	"update",
	"refactor",
	"clean",
	"remove",
	"rename",
	"move",
}

// InferCommitType analyzes the task title and returns an appropriate CommitType.
// It uses keyword matching to determine the type:
// - "add", "implement", "create", "new" -> feat
// - "fix", "repair", "resolve", "correct" -> fix
// - "update", "refactor", "clean", "remove", "rename", "move" -> chore
// If no keyword matches, defaults to chore.
func InferCommitType(title string) CommitType {
	titleLower := strings.ToLower(title)

	// Check for feat keywords
	for _, keyword := range featKeywords {
		if strings.HasPrefix(titleLower, keyword+" ") || strings.HasPrefix(titleLower, keyword+":") {
			return CommitTypeFeat
		}
	}

	// Check for fix keywords
	for _, keyword := range fixKeywords {
		if strings.HasPrefix(titleLower, keyword+" ") || strings.HasPrefix(titleLower, keyword+":") {
			return CommitTypeFix
		}
	}

	// Check for chore keywords
	for _, keyword := range choreKeywords {
		if strings.HasPrefix(titleLower, keyword+" ") || strings.HasPrefix(titleLower, keyword+":") {
			return CommitTypeChore
		}
	}

	// Default to chore
	return CommitTypeChore
}

// FormatCommitMessage creates a conventional commit message from a task title
// and iteration ID. The commit type is inferred from the title.
// Format: "<type>: <title>\n\nRalph iteration: <iterationID>"
// If iterationID is empty, the body is omitted.
func FormatCommitMessage(taskTitle, iterationID string) string {
	commitType := InferCommitType(taskTitle)
	return FormatCommitMessageWithType(commitType, taskTitle, iterationID)
}

// FormatCommitMessageWithType creates a conventional commit message with an
// explicit commit type. Use this when you want to override the inferred type.
// Format: "<type>: <title>\n\nRalph iteration: <iterationID>"
// If iterationID is empty, the body is omitted.
func FormatCommitMessageWithType(commitType CommitType, taskTitle, iterationID string) string {
	subject := fmt.Sprintf("%s: %s", commitType, taskTitle)

	if iterationID == "" {
		return subject
	}

	return fmt.Sprintf("%s\n\nRalph iteration: %s", subject, iterationID)
}

// ParseConventionalCommit parses a conventional commit message and returns
// the commit type, subject, and body. Returns empty values if the message
// doesn't follow the conventional commit format.
func ParseConventionalCommit(message string) (commitType CommitType, subject, body string) {
	if message == "" {
		return "", "", ""
	}

	// Split message into lines
	lines := strings.SplitN(message, "\n", 2)
	firstLine := lines[0]

	// Parse the first line for type and subject
	typeStr, subjectStr, found := strings.Cut(firstLine, ":")
	if !found {
		return "", "", ""
	}

	typeStr = strings.TrimSpace(typeStr)
	subject = strings.TrimSpace(subjectStr)

	ct := CommitType(typeStr)
	if !ct.IsValid() {
		return "", "", ""
	}

	// Extract body if present (skip blank line between subject and body)
	if len(lines) > 1 {
		body = strings.TrimPrefix(lines[1], "\n")
	}

	return ct, subject, body
}

// ValidateCommitMessage validates that a commit message follows the
// conventional commit format with a supported type.
func ValidateCommitMessage(message string) error {
	if message == "" {
		return fmt.Errorf("commit message is empty")
	}

	lines := strings.SplitN(message, "\n", 2)
	firstLine := lines[0]

	typeStr, subjectStr, found := strings.Cut(firstLine, ":")
	if !found {
		return fmt.Errorf("commit message missing colon separator")
	}

	typeStr = strings.TrimSpace(typeStr)
	if typeStr == "" {
		return fmt.Errorf("commit message missing type prefix")
	}

	ct := CommitType(typeStr)
	if !ct.IsValid() {
		return fmt.Errorf("commit message has unknown type: %q", typeStr)
	}

	subject := strings.TrimSpace(subjectStr)
	if subject == "" {
		return fmt.Errorf("commit message has empty subject")
	}

	return nil
}
