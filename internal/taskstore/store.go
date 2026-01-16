package taskstore

import (
	"errors"
	"fmt"
)

// Error types for TaskStore operations.
var (
	// ErrNotFound is returned when a task with the given ID does not exist.
	ErrNotFound = errors.New("task not found")

	// ErrValidation is returned when a task fails validation.
	ErrValidation = errors.New("task validation failed")
)

// NotFoundError wraps ErrNotFound with the task ID that was not found.
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("task not found: %s", e.ID)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// ValidationError wraps ErrValidation with details about the validation failure.
type ValidationError struct {
	ID     string
	Reason string
}

func (e *ValidationError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("task validation failed for %s: %s", e.ID, e.Reason)
	}
	return fmt.Sprintf("task validation failed: %s", e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

// Store defines the interface for task persistence and retrieval.
// This interface is defined at the consumer level following Go idioms.
type Store interface {
	// Get retrieves a task by its ID.
	// Returns NotFoundError if the task does not exist.
	Get(id string) (*Task, error)

	// List retrieves all tasks.
	List() ([]*Task, error)

	// ListByParent retrieves all tasks with the given parent ID.
	// If parentID is empty, returns tasks with no parent (root tasks).
	ListByParent(parentID string) ([]*Task, error)

	// Save persists a task. If a task with the same ID exists, it is updated.
	// Returns ValidationError if the task fails validation.
	Save(task *Task) error

	// UpdateStatus updates only the status of an existing task.
	// Returns NotFoundError if the task does not exist.
	UpdateStatus(id string, status TaskStatus) error

	// Delete removes a task by its ID.
	// Returns NotFoundError if the task does not exist.
	Delete(id string) error
}
