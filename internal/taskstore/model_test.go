package taskstore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskStatus_ValidValues(t *testing.T) {
	validStatuses := []TaskStatus{
		StatusOpen,
		StatusInProgress,
		StatusCompleted,
		StatusBlocked,
		StatusFailed,
		StatusSkipped,
	}

	for _, status := range validStatuses {
		assert.True(t, status.IsValid(), "status %q should be valid", status)
	}
}

func TestTaskStatus_InvalidValue(t *testing.T) {
	invalid := TaskStatus("invalid")
	assert.False(t, invalid.IsValid())
}

func TestTaskStatus_String(t *testing.T) {
	assert.Equal(t, "open", string(StatusOpen))
	assert.Equal(t, "in_progress", string(StatusInProgress))
	assert.Equal(t, "completed", string(StatusCompleted))
	assert.Equal(t, "blocked", string(StatusBlocked))
	assert.Equal(t, "failed", string(StatusFailed))
	assert.Equal(t, "skipped", string(StatusSkipped))
}

func TestTask_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	parentID := "parent-task"
	task := &Task{
		ID:          "task-1",
		Title:       "Test Task",
		Description: "A test task description",
		ParentID:    &parentID,
		DependsOn:   []string{"dep-1", "dep-2"},
		Status:      StatusOpen,
		Acceptance:  []string{"acceptance 1", "acceptance 2"},
		Verify:      [][]string{{"go", "test", "./..."}},
		Labels:      map[string]string{"area": "core"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Serialize to JSON
	data, err := json.Marshal(task)
	require.NoError(t, err)

	// Deserialize back
	var decoded Task
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, task.ID, decoded.ID)
	assert.Equal(t, task.Title, decoded.Title)
	assert.Equal(t, task.Description, decoded.Description)
	assert.Equal(t, *task.ParentID, *decoded.ParentID)
	assert.Equal(t, task.DependsOn, decoded.DependsOn)
	assert.Equal(t, task.Status, decoded.Status)
	assert.Equal(t, task.Acceptance, decoded.Acceptance)
	assert.Equal(t, task.Verify, decoded.Verify)
	assert.Equal(t, task.Labels, decoded.Labels)
	assert.True(t, task.CreatedAt.Equal(decoded.CreatedAt))
	assert.True(t, task.UpdatedAt.Equal(decoded.UpdatedAt))
}

func TestTask_JSONSerialization_NilParentID(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(task)
	require.NoError(t, err)

	var decoded Task
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Nil(t, decoded.ParentID)
}

func TestTask_Validate_Valid(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := task.Validate()
	assert.NoError(t, err)
}

func TestTask_Validate_MissingID(t *testing.T) {
	task := &Task{
		Title:     "Test Task",
		Status:    StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := task.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestTask_Validate_MissingTitle(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Status:    StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := task.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestTask_Validate_InvalidStatus(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    TaskStatus("invalid"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := task.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestTask_Validate_EmptyStatus(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Title:     "Test Task",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := task.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestTask_Validate_ZeroCreatedAt(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    StatusOpen,
		UpdatedAt: time.Now(),
	}

	err := task.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "created_at")
}

func TestTask_Validate_ZeroUpdatedAt(t *testing.T) {
	task := &Task{
		ID:        "task-1",
		Title:     "Test Task",
		Status:    StatusOpen,
		CreatedAt: time.Now(),
	}

	err := task.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updated_at")
}

func TestTask_Validate_AllFields(t *testing.T) {
	parentID := "parent-task"
	task := &Task{
		ID:          "task-1",
		Title:       "Test Task",
		Description: "A detailed description",
		ParentID:    &parentID,
		DependsOn:   []string{"dep-1"},
		Status:      StatusCompleted,
		Acceptance:  []string{"test passes"},
		Verify:      [][]string{{"go", "test"}},
		Labels:      map[string]string{"area": "core"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := task.Validate()
	assert.NoError(t, err)
}
