// Package loop provides iteration orchestration for the Ralph harness.
package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// AttemptNumber is the retry attempt number (1 for first attempt, 2 for first retry, etc.).
	AttemptNumber int `json:"attempt_number,omitempty"`
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

// GenerateTextLog generates a human-readable text summary of an iteration record.
func GenerateTextLog(record *IterationRecord) string {
	if record == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Iteration: %s\n", record.IterationID))
	sb.WriteString(fmt.Sprintf("Task: %s\n", record.TaskID))

	// Timing
	if !record.StartTime.IsZero() {
		sb.WriteString(fmt.Sprintf("Start Time: %s\n", record.StartTime.Format(time.RFC3339)))
	}
	if !record.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("End Time: %s\n", record.EndTime.Format(time.RFC3339)))
	}
	if duration := record.Duration(); duration > 0 {
		sb.WriteString(fmt.Sprintf("Duration: %s\n", duration))
	}

	// Outcome
	sb.WriteString(fmt.Sprintf("Outcome: %s\n", record.Outcome))

	// Commits
	if record.BaseCommit != "" {
		sb.WriteString(fmt.Sprintf("Base Commit: %s\n", record.BaseCommit))
	}
	if record.ResultCommit != "" {
		sb.WriteString(fmt.Sprintf("Commit: %s\n", record.ResultCommit))
	}

	// Files changed
	if len(record.FilesChanged) > 0 {
		sb.WriteString("\nFiles Changed:\n")
		for _, file := range record.FilesChanged {
			sb.WriteString(fmt.Sprintf("  - %s\n", file))
		}
	}

	// Verification results
	if len(record.VerificationOutputs) > 0 {
		sb.WriteString("\nVerification Results:\n")
		for _, vo := range record.VerificationOutputs {
			status := "PASS"
			if !vo.Passed {
				status = "FAIL"
			}
			cmdStr := strings.Join(vo.Command, " ")
			sb.WriteString(fmt.Sprintf("  - %s - %s\n", cmdStr, status))
			if vo.Duration > 0 {
				sb.WriteString(fmt.Sprintf("    Duration: %s\n", vo.Duration))
			}
		}
	}

	// Claude invocation metadata
	if record.ClaudeInvocation.Model != "" || record.ClaudeInvocation.SessionID != "" {
		sb.WriteString("\nClaude Invocation:\n")
		if record.ClaudeInvocation.Model != "" {
			sb.WriteString(fmt.Sprintf("  Model: %s\n", record.ClaudeInvocation.Model))
		}
		if record.ClaudeInvocation.SessionID != "" {
			sb.WriteString(fmt.Sprintf("  Session ID: %s\n", record.ClaudeInvocation.SessionID))
		}
		if record.ClaudeInvocation.TotalCostUSD > 0 {
			sb.WriteString(fmt.Sprintf("  Cost: $%.4f\n", record.ClaudeInvocation.TotalCostUSD))
		}
		if record.ClaudeInvocation.InputTokens > 0 {
			sb.WriteString(fmt.Sprintf("  Input Tokens: %d\n", record.ClaudeInvocation.InputTokens))
		}
		if record.ClaudeInvocation.OutputTokens > 0 {
			sb.WriteString(fmt.Sprintf("  Output Tokens: %d\n", record.ClaudeInvocation.OutputTokens))
		}
	}

	// Feedback (for retries)
	if record.Feedback != "" {
		sb.WriteString(fmt.Sprintf("\nFeedback: %s\n", record.Feedback))
	}

	// Attempt number (for retries)
	if record.AttemptNumber > 0 {
		sb.WriteString(fmt.Sprintf("Attempt Number: %d\n", record.AttemptNumber))
	}

	return sb.String()
}

// SaveRecord saves an iteration record to the logs directory.
// Returns the path to the saved JSON file.
// Also creates a human-readable text log file.
func SaveRecord(logsDir string, record *IterationRecord) (string, error) {
	if record == nil {
		return "", errors.New("record cannot be nil")
	}

	// Ensure directory exists
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate filenames
	jsonFilename := fmt.Sprintf("iteration-%s.json", record.IterationID)
	textFilename := fmt.Sprintf("iteration-%s.txt", record.IterationID)
	jsonPath := filepath.Join(logsDir, jsonFilename)
	textPath := filepath.Join(logsDir, textFilename)

	// Marshal to JSON
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal record: %w", err)
	}

	// Write JSON file
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write record: %w", err)
	}

	// Generate and write text log
	textLog := GenerateTextLog(record)
	if err := os.WriteFile(textPath, []byte(textLog), 0644); err != nil {
		// Don't fail if text log write fails, just log the error
		// JSON is the source of truth
		fmt.Fprintf(os.Stderr, "warning: failed to write text log: %v\n", err)
	}

	return jsonPath, nil
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

// LoadAllIterationRecords loads all iteration records from the logs directory.
func LoadAllIterationRecords(logsDir string) ([]*IterationRecord, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read logs directory: %w", err)
	}

	var records []*IterationRecord
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process iteration JSON files
		name := entry.Name()
		if len(name) < 10 || name[:10] != "iteration-" || filepath.Ext(name) != ".json" {
			continue
		}

		path := filepath.Join(logsDir, name)
		record, err := LoadRecord(path)
		if err != nil {
			continue // Skip invalid files
		}

		records = append(records, record)
	}

	return records, nil
}
