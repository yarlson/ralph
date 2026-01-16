package taskstore

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotFoundError(t *testing.T) {
	t.Run("wraps ErrNotFound", func(t *testing.T) {
		err := &NotFoundError{ID: "task-123"}

		assert.True(t, errors.Is(err, ErrNotFound))
	})

	t.Run("error message includes task ID", func(t *testing.T) {
		err := &NotFoundError{ID: "my-task-id"}

		assert.Equal(t, "task not found: my-task-id", err.Error())
	})

	t.Run("can be unwrapped with errors.As", func(t *testing.T) {
		err := &NotFoundError{ID: "task-456"}

		var notFound *NotFoundError
		require.True(t, errors.As(err, &notFound))
		assert.Equal(t, "task-456", notFound.ID)
	})

	t.Run("wrapped error can be detected", func(t *testing.T) {
		inner := &NotFoundError{ID: "inner-task"}
		wrapped := errors.Join(errors.New("outer error"), inner)

		assert.True(t, errors.Is(wrapped, ErrNotFound))

		var notFound *NotFoundError
		require.True(t, errors.As(wrapped, &notFound))
		assert.Equal(t, "inner-task", notFound.ID)
	})
}

func TestValidationError(t *testing.T) {
	t.Run("wraps ErrValidation", func(t *testing.T) {
		err := &ValidationError{ID: "task-123", Reason: "invalid status"}

		assert.True(t, errors.Is(err, ErrValidation))
	})

	t.Run("error message with ID includes task ID and reason", func(t *testing.T) {
		err := &ValidationError{ID: "my-task", Reason: "title is required"}

		assert.Equal(t, "task validation failed for my-task: title is required", err.Error())
	})

	t.Run("error message without ID includes only reason", func(t *testing.T) {
		err := &ValidationError{Reason: "invalid task data"}

		assert.Equal(t, "task validation failed: invalid task data", err.Error())
	})

	t.Run("can be unwrapped with errors.As", func(t *testing.T) {
		err := &ValidationError{ID: "task-789", Reason: "bad status"}

		var validationErr *ValidationError
		require.True(t, errors.As(err, &validationErr))
		assert.Equal(t, "task-789", validationErr.ID)
		assert.Equal(t, "bad status", validationErr.Reason)
	})

	t.Run("wrapped error can be detected", func(t *testing.T) {
		inner := &ValidationError{ID: "inner-task", Reason: "missing field"}
		wrapped := errors.Join(errors.New("outer error"), inner)

		assert.True(t, errors.Is(wrapped, ErrValidation))

		var validationErr *ValidationError
		require.True(t, errors.As(wrapped, &validationErr))
		assert.Equal(t, "inner-task", validationErr.ID)
	})
}

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrNotFound and ErrValidation are distinct", func(t *testing.T) {
		assert.False(t, errors.Is(ErrNotFound, ErrValidation))
		assert.False(t, errors.Is(ErrValidation, ErrNotFound))
	})

	t.Run("ErrNotFound has expected message", func(t *testing.T) {
		assert.Equal(t, "task not found", ErrNotFound.Error())
	})

	t.Run("ErrValidation has expected message", func(t *testing.T) {
		assert.Equal(t, "task validation failed", ErrValidation.Error())
	})
}
