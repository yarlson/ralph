// Package loop provides iteration orchestration for the Ralph harness.
package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// IterationOutcome represents the result of an iteration.
type IterationOutcome string

const (
	// OutcomeSuccess indicates the iteration completed successfully with verified commit.
	OutcomeSuccess IterationOutcome = "success"
	// OutcomeFailed indicates the iteration failed verification.
	OutcomeFailed IterationOutcome = "failed"
	// OutcomeBudgetExceeded indicates the iteration exceeded budget limits.
	OutcomeBudgetExceeded IterationOutcome = "budget_exceeded"
	// OutcomeBlocked indicates the iteration was blocked (e.g., no ready tasks).
	OutcomeBlocked IterationOutcome = "blocked"
)

// validOutcomes is a set of valid iteration outcomes for validation.
var validOutcomes = map[IterationOutcome]bool{
	OutcomeSuccess:        true,
	OutcomeFailed:         true,
	OutcomeBudgetExceeded: true,
	OutcomeBlocked:        true,
}

// IsValid returns true if the outcome is a valid value.
func (o IterationOutcome) IsValid() bool {
	return validOutcomes[o]
}

// IterationRecord contains all information about a single iteration execution.
// This is the primary audit record for each iteration in the Ralph loop.
type IterationRecord struct {
	// IterationID is the unique identifier for this iteration.
	IterationID string `json:"iteration_id"`

	// TaskID is the ID of the task being executed in this iteration.
	TaskID string `json:"task_id"`

	// StartTime is when the iteration started.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the iteration completed.
	EndTime time.Time `json:"end_time"`

	// ClaudeInvocation contains metadata about the Claude Code invocation.
	ClaudeInvocation ClaudeInvocationMeta `json:"claude_invocation"`

	// BaseCommit is the git commit hash at the start of the iteration.
	BaseCommit string `json:"base_commit,omitempty"`

	// ResultCommit is the git commit hash after successful completion.
	ResultCommit string `json:"result_commit,omitempty"`

	// VerificationOutputs contains the results of verification commands.
	VerificationOutputs []VerificationOutput `json:"verification_outputs,omitempty"`

	// FilesChanged lists the files modified during this iteration.
	FilesChanged []string `json:"files_changed,omitempty"`

	// Outcome is the final result of the iteration.
	Outcome IterationOutcome `json:"outcome"`

	// Feedback contains failure information or user feedback for retry.
	Feedback string `json:"feedback,omitempty"`
}

// ClaudeInvocationMeta contains metadata about a Claude Code invocation.
type ClaudeInvocationMeta struct {
	// Command is the CLI command that was executed.
	Command []string `json:"command,omitempty"`

	// Model is the Claude model used (e.g., "claude-3-sonnet").
	Model string `json:"model,omitempty"`

	// SessionID is the Claude session identifier.
	SessionID string `json:"session_id,omitempty"`

	// TotalCostUSD is the cost of this invocation for budget tracking.
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`

	// InputTokens is the number of input tokens consumed.
	InputTokens int `json:"input_tokens,omitempty"`

	// OutputTokens is the number of output tokens generated.
	OutputTokens int `json:"output_tokens,omitempty"`
}

// VerificationOutput contains the result of a single verification command.
type VerificationOutput struct {
	// Command is the verification command that was executed.
	Command []string `json:"command"`

	// Passed indicates whether the command succeeded (exit code 0).
	Passed bool `json:"passed"`

	// Output is the combined stdout/stderr from the command.
	Output string `json:"output,omitempty"`

	// Duration is how long the command took to execute.
	Duration time.Duration `json:"duration,omitempty"`
}

// NewIterationRecord creates a new iteration record for the given task.
// It generates a unique iteration ID and sets the start time.
func NewIterationRecord(taskID string) *IterationRecord {
	return &IterationRecord{
		IterationID: GenerateIterationID(),
		TaskID:      taskID,
		StartTime:   time.Now(),
	}
}

// Duration returns the duration of the iteration.
func (r *IterationRecord) Duration() time.Duration {
	if r.StartTime.IsZero() || r.EndTime.IsZero() {
		return 0
	}
	return r.EndTime.Sub(r.StartTime)
}

// Complete marks the iteration as complete with the given outcome.
func (r *IterationRecord) Complete(outcome IterationOutcome) {
	r.EndTime = time.Now()
	r.Outcome = outcome
}

// SetFeedback sets the feedback for retry attempts.
func (r *IterationRecord) SetFeedback(feedback string) {
	r.Feedback = feedback
}

// AllPassed returns true if all verification outputs passed.
func (r *IterationRecord) AllPassed() bool {
	for _, output := range r.VerificationOutputs {
		if !output.Passed {
			return false
		}
	}
	return true
}

// GenerateIterationID generates a unique iteration ID.
func GenerateIterationID() string {
	return uuid.New().String()[:8]
}

// SaveRecord saves an iteration record to the logs directory.
// Returns the path to the saved file.
func SaveRecord(logsDir string, record *IterationRecord) (string, error) {
	if record == nil {
		return "", errors.New("record cannot be nil")
	}

	// Ensure directory exists
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("iteration-%s.json", record.IterationID)
	path := filepath.Join(logsDir, filename)

	// Marshal to JSON
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal record: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write record: %w", err)
	}

	return path, nil
}

// LoadRecord loads an iteration record from a file.
func LoadRecord(path string) (*IterationRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read record: %w", err)
	}

	var record IterationRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return &record, nil
}
