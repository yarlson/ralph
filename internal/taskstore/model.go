// Package taskstore provides task persistence and retrieval for the Ralph harness.
package taskstore

import (
	"fmt"
	"time"
)

// TaskStatus represents the current state of a task.
type TaskStatus string

// Valid task status values.
const (
	StatusOpen       TaskStatus = "open"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusBlocked    TaskStatus = "blocked"
	StatusFailed     TaskStatus = "failed"
	StatusSkipped    TaskStatus = "skipped"
)

// validStatuses contains all valid status values for quick lookup.
var validStatuses = map[TaskStatus]bool{
	StatusOpen:       true,
	StatusInProgress: true,
	StatusCompleted:  true,
	StatusBlocked:    true,
	StatusFailed:     true,
	StatusSkipped:    true,
}

// IsValid returns true if the status is a valid TaskStatus value.
func (s TaskStatus) IsValid() bool {
	return validStatuses[s]
}

// Task represents a unit of work in the Ralph task hierarchy.
type Task struct {
	// ID is the unique identifier for the task.
	ID string `json:"id"`

	// Title is the short summary of the task.
	Title string `json:"title"`

	// Description is the detailed standalone description of the task.
	Description string `json:"description,omitempty"`

	// ParentID is the optional ID of the parent task.
	ParentID *string `json:"parent_id,omitempty"`

	// DependsOn lists task IDs that must be completed before this task.
	DependsOn []string `json:"depends_on,omitempty"`

	// Status is the current state of the task.
	Status TaskStatus `json:"status"`

	// Acceptance lists the verifiable acceptance criteria for the task.
	Acceptance []string `json:"acceptance,omitempty"`

	// Verify lists the commands to run for verification (e.g., [["go", "test", "./..."]]).
	Verify [][]string `json:"verify,omitempty"`

	// Labels is a map of key-value pairs for categorization (e.g., {"area": "core"}).
	Labels map[string]string `json:"labels,omitempty"`

	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the task was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that the task has all required fields and valid values.
// Returns an error describing the first validation failure, or nil if valid.
func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task id is required")
	}

	if t.Title == "" {
		return fmt.Errorf("task title is required")
	}

	if !t.Status.IsValid() {
		return fmt.Errorf("task status is invalid: %q", t.Status)
	}

	if t.CreatedAt.IsZero() {
		return fmt.Errorf("task created_at is required")
	}

	if t.UpdatedAt.IsZero() {
		return fmt.Errorf("task updated_at is required")
	}

	return nil
}
