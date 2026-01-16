package reporter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/ralph/internal/loop"
	"github.com/yarlson/ralph/internal/taskstore"
)

func TestTaskCounts(t *testing.T) {
	t.Run("zero values", func(t *testing.T) {
		counts := TaskCounts{}
		assert.Equal(t, 0, counts.Total)
		assert.Equal(t, 0, counts.Completed)
		assert.Equal(t, 0, counts.Ready)
		assert.Equal(t, 0, counts.Blocked)
		assert.Equal(t, 0, counts.Failed)
		assert.Equal(t, 0, counts.Skipped)
	})

	t.Run("all fields", func(t *testing.T) {
		counts := TaskCounts{
			Total:     10,
			Completed: 5,
			Ready:     2,
			Blocked:   1,
			Failed:    1,
			Skipped:   1,
		}
		assert.Equal(t, 10, counts.Total)
		assert.Equal(t, 5, counts.Completed)
		assert.Equal(t, 2, counts.Ready)
		assert.Equal(t, 1, counts.Blocked)
		assert.Equal(t, 1, counts.Failed)
		assert.Equal(t, 1, counts.Skipped)
	})
}

func TestStatus(t *testing.T) {
	t.Run("zero values", func(t *testing.T) {
		status := Status{}
		assert.Equal(t, "", status.ParentTaskID)
		assert.Equal(t, TaskCounts{}, status.Counts)
		assert.Nil(t, status.NextTask)
		assert.Nil(t, status.LastIteration)
	})

	t.Run("all fields", func(t *testing.T) {
		nextTask := &taskstore.Task{ID: "task-1", Title: "Test Task"}
		lastIter := &LastIterationInfo{
			IterationID: "abc12345",
			TaskID:      "task-0",
			TaskTitle:   "Previous Task",
			Outcome:     loop.OutcomeSuccess,
			EndTime:     time.Now(),
		}

		status := Status{
			ParentTaskID: "parent-1",
			Counts: TaskCounts{
				Total:     5,
				Completed: 2,
			},
			NextTask:      nextTask,
			LastIteration: lastIter,
		}

		assert.Equal(t, "parent-1", status.ParentTaskID)
		assert.Equal(t, 5, status.Counts.Total)
		assert.Equal(t, "task-1", status.NextTask.ID)
		assert.Equal(t, "abc12345", status.LastIteration.IterationID)
	})
}

func TestLastIterationInfo(t *testing.T) {
	t.Run("zero values", func(t *testing.T) {
		info := LastIterationInfo{}
		assert.Equal(t, "", info.IterationID)
		assert.Equal(t, "", info.TaskID)
		assert.Equal(t, "", info.TaskTitle)
		assert.Equal(t, loop.IterationOutcome(""), info.Outcome)
		assert.True(t, info.EndTime.IsZero())
		assert.Equal(t, "", info.LogPath)
	})

	t.Run("all fields", func(t *testing.T) {
		endTime := time.Now()
		info := LastIterationInfo{
			IterationID: "iter-123",
			TaskID:      "task-42",
			TaskTitle:   "Build Feature",
			Outcome:     loop.OutcomeFailed,
			EndTime:     endTime,
			LogPath:     "/logs/iteration-iter-123.json",
		}

		assert.Equal(t, "iter-123", info.IterationID)
		assert.Equal(t, "task-42", info.TaskID)
		assert.Equal(t, "Build Feature", info.TaskTitle)
		assert.Equal(t, loop.OutcomeFailed, info.Outcome)
		assert.Equal(t, endTime, info.EndTime)
		assert.Equal(t, "/logs/iteration-iter-123.json", info.LogPath)
	})
}

func TestNewStatusGenerator(t *testing.T) {
	store := &mockTaskStore{}
	gen := NewStatusGenerator(store, "")

	assert.NotNil(t, gen)
	assert.Equal(t, store, gen.taskStore)
	assert.Equal(t, "", gen.logsDir)
}

func TestStatusGenerator_GetStatus(t *testing.T) {
	t.Run("no tasks", func(t *testing.T) {
		store := &mockTaskStore{
			tasks: []*taskstore.Task{},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.Equal(t, "parent-1", status.ParentTaskID)
		assert.Equal(t, 0, status.Counts.Total)
		assert.Nil(t, status.NextTask)
		assert.Nil(t, status.LastIteration)
	})

	t.Run("with tasks under parent", func(t *testing.T) {
		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusCompleted, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-2", Title: "Task 2", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-3", Title: "Task 3", Status: taskstore.StatusFailed, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.Equal(t, "parent-1", status.ParentTaskID)
		assert.Equal(t, 3, status.Counts.Total)
		assert.Equal(t, 1, status.Counts.Completed)
		assert.Equal(t, 1, status.Counts.Failed)
		// task-2 should be ready (no dependencies)
		assert.NotNil(t, status.NextTask)
		assert.Equal(t, "task-2", status.NextTask.ID)
	})

	t.Run("with blocked tasks", func(t *testing.T) {
		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusOpen, ParentID: &parentID, DependsOn: []string{"task-2"}, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-2", Title: "Task 2", Status: taskstore.StatusBlocked, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.Equal(t, 2, status.Counts.Total)
		assert.Equal(t, 1, status.Counts.Blocked)
		// Both tasks should not be ready (task-1 depends on task-2, task-2 is blocked)
		assert.Nil(t, status.NextTask)
	})

	t.Run("with skipped tasks", func(t *testing.T) {
		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusSkipped, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.Equal(t, 1, status.Counts.Total)
		assert.Equal(t, 1, status.Counts.Skipped)
	})

	t.Run("counts ready tasks", func(t *testing.T) {
		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-2", Title: "Task 2", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-3", Title: "Task 3", Status: taskstore.StatusOpen, ParentID: &parentID, DependsOn: []string{"task-1"}, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		// task-1 and task-2 are ready (no dependencies), task-3 is not ready (depends on task-1)
		assert.Equal(t, 3, status.Counts.Total)
		assert.Equal(t, 2, status.Counts.Ready)
	})

	t.Run("handles deep hierarchy", func(t *testing.T) {
		parentID := "parent-1"
		subParentID := "sub-parent"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "sub-parent", Title: "Sub Parent", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "leaf-1", Title: "Leaf 1", Status: taskstore.StatusCompleted, ParentID: &subParentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "leaf-2", Title: "Leaf 2", Status: taskstore.StatusOpen, ParentID: &subParentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		// Should count all descendants: sub-parent, leaf-1, leaf-2
		assert.Equal(t, 3, status.Counts.Total)
		assert.Equal(t, 1, status.Counts.Completed)
	})

	t.Run("store error", func(t *testing.T) {
		store := &mockTaskStore{
			listErr: assert.AnError,
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		assert.Error(t, err)
		assert.Nil(t, status)
	})
}

func TestStatusGenerator_GetStatus_WithLastIteration(t *testing.T) {
	t.Run("loads last iteration from logs", func(t *testing.T) {
		logsDir := t.TempDir()

		// Create a test iteration record
		parentID := "parent-1"
		record := &loop.IterationRecord{
			IterationID: "test-iter",
			TaskID:      "task-1",
			StartTime:   time.Now().Add(-time.Minute),
			EndTime:     time.Now(),
			Outcome:     loop.OutcomeSuccess,
		}
		_, err := loop.SaveRecord(logsDir, record)
		require.NoError(t, err)

		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusCompleted, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, logsDir)

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.NotNil(t, status.LastIteration)
		assert.Equal(t, "test-iter", status.LastIteration.IterationID)
		assert.Equal(t, "task-1", status.LastIteration.TaskID)
		assert.Equal(t, loop.OutcomeSuccess, status.LastIteration.Outcome)
		assert.Contains(t, status.LastIteration.LogPath, "iteration-test-iter.json")
	})

	t.Run("no iteration logs", func(t *testing.T) {
		logsDir := t.TempDir()

		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, logsDir)

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.Nil(t, status.LastIteration)
	})

	t.Run("empty logs dir", func(t *testing.T) {
		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.Nil(t, status.LastIteration)
	})

	t.Run("picks latest iteration by end time", func(t *testing.T) {
		logsDir := t.TempDir()

		// Create two iteration records
		parentID := "parent-1"
		oldRecord := &loop.IterationRecord{
			IterationID: "old-iter",
			TaskID:      "task-1",
			StartTime:   time.Now().Add(-2 * time.Hour),
			EndTime:     time.Now().Add(-time.Hour),
			Outcome:     loop.OutcomeSuccess,
		}
		_, err := loop.SaveRecord(logsDir, oldRecord)
		require.NoError(t, err)

		newRecord := &loop.IterationRecord{
			IterationID: "new-iter",
			TaskID:      "task-2",
			StartTime:   time.Now().Add(-30 * time.Minute),
			EndTime:     time.Now().Add(-10 * time.Minute),
			Outcome:     loop.OutcomeFailed,
		}
		_, err = loop.SaveRecord(logsDir, newRecord)
		require.NoError(t, err)

		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusCompleted, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-2", Title: "Task 2", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, logsDir)

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.NotNil(t, status.LastIteration)
		assert.Equal(t, "new-iter", status.LastIteration.IterationID)
		assert.Equal(t, loop.OutcomeFailed, status.LastIteration.Outcome)
	})
}

func TestStatusGenerator_Format(t *testing.T) {
	t.Run("formats basic status", func(t *testing.T) {
		parentID := "parent-1"
		nextTask := &taskstore.Task{ID: "task-2", Title: "Next Task"}
		status := &Status{
			ParentTaskID: parentID,
			Counts: TaskCounts{
				Total:     5,
				Completed: 2,
				Ready:     2,
				Blocked:   0,
				Failed:    1,
				Skipped:   0,
			},
			NextTask: nextTask,
		}

		formatted := FormatStatus(status)

		assert.Contains(t, formatted, "Parent: parent-1")
		assert.Contains(t, formatted, "Total: 5")
		assert.Contains(t, formatted, "Completed: 2")
		assert.Contains(t, formatted, "Ready: 2")
		assert.Contains(t, formatted, "Failed: 1")
		assert.Contains(t, formatted, "Next Task: task-2")
		assert.Contains(t, formatted, "Next Task")
	})

	t.Run("formats status with last iteration", func(t *testing.T) {
		lastIter := &LastIterationInfo{
			IterationID: "abc12345",
			TaskID:      "task-1",
			TaskTitle:   "Previous Task",
			Outcome:     loop.OutcomeSuccess,
			EndTime:     time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			LogPath:     "/logs/iteration-abc12345.json",
		}
		status := &Status{
			ParentTaskID:  "parent-1",
			Counts:        TaskCounts{Total: 3, Completed: 1},
			LastIteration: lastIter,
		}

		formatted := FormatStatus(status)

		assert.Contains(t, formatted, "Last Iteration")
		assert.Contains(t, formatted, "abc12345")
		assert.Contains(t, formatted, "success")
		assert.Contains(t, formatted, "task-1")
	})

	t.Run("formats status with no next task", func(t *testing.T) {
		status := &Status{
			ParentTaskID: "parent-1",
			Counts:       TaskCounts{Total: 2, Completed: 2},
			NextTask:     nil,
		}

		formatted := FormatStatus(status)

		assert.Contains(t, formatted, "Next Task: none")
	})

	t.Run("formats empty status", func(t *testing.T) {
		status := &Status{
			ParentTaskID: "parent-1",
			Counts:       TaskCounts{},
		}

		formatted := FormatStatus(status)

		assert.Contains(t, formatted, "Parent: parent-1")
		assert.Contains(t, formatted, "Total: 0")
	})

	t.Run("formats status with next task feedback", func(t *testing.T) {
		nextTask := &taskstore.Task{ID: "task-2", Title: "Next Task"}
		status := &Status{
			ParentTaskID:     "parent-1",
			Counts:           TaskCounts{Total: 3, Completed: 1},
			NextTask:         nextTask,
			NextTaskFeedback: "Try a different approach",
		}

		formatted := FormatStatus(status)

		assert.Contains(t, formatted, "Next Task: task-2")
		assert.Contains(t, formatted, "Feedback: Try a different approach")
	})
}

func TestFindLatestIterationRecord(t *testing.T) {
	t.Run("finds record in directory", func(t *testing.T) {
		logsDir := t.TempDir()

		record := &loop.IterationRecord{
			IterationID: "test-123",
			TaskID:      "task-1",
			StartTime:   time.Now().Add(-time.Minute),
			EndTime:     time.Now(),
			Outcome:     loop.OutcomeSuccess,
		}
		_, err := loop.SaveRecord(logsDir, record)
		require.NoError(t, err)

		found, path, err := FindLatestIterationRecord(logsDir)
		require.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, "test-123", found.IterationID)
		assert.Equal(t, filepath.Join(logsDir, "iteration-test-123.json"), path)
	})

	t.Run("returns nil for empty directory", func(t *testing.T) {
		logsDir := t.TempDir()

		found, path, err := FindLatestIterationRecord(logsDir)
		require.NoError(t, err)
		assert.Nil(t, found)
		assert.Equal(t, "", path)
	})

	t.Run("returns nil for non-existent directory", func(t *testing.T) {
		found, path, err := FindLatestIterationRecord("/non/existent/path")
		require.NoError(t, err)
		assert.Nil(t, found)
		assert.Equal(t, "", path)
	})
}

func TestStatusGenerator_GetStatus_WithFeedback(t *testing.T) {
	t.Run("loads feedback for next task", func(t *testing.T) {
		stateDir := t.TempDir()

		// Create feedback file for task-2
		feedbackPath := filepath.Join(stateDir, "feedback-task-2.txt")
		err := os.WriteFile(feedbackPath, []byte("Try approach X"), 0644)
		require.NoError(t, err)

		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusCompleted, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-2", Title: "Task 2", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGeneratorWithStateDir(store, "", stateDir)

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.NotNil(t, status.NextTask)
		assert.Equal(t, "task-2", status.NextTask.ID)
		assert.Equal(t, "Try approach X", status.NextTaskFeedback)
	})

	t.Run("handles missing feedback file gracefully", func(t *testing.T) {
		stateDir := t.TempDir()

		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGeneratorWithStateDir(store, "", stateDir)

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.NotNil(t, status.NextTask)
		assert.Equal(t, "", status.NextTaskFeedback)
	})

	t.Run("no feedback when no state dir", func(t *testing.T) {
		parentID := "parent-1"
		store := &mockTaskStore{
			tasks: []*taskstore.Task{
				{ID: "parent-1", Title: "Parent", Status: taskstore.StatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: "task-1", Title: "Task 1", Status: taskstore.StatusOpen, ParentID: &parentID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		}
		gen := NewStatusGenerator(store, "")

		status, err := gen.GetStatus("parent-1")
		require.NoError(t, err)

		assert.NotNil(t, status.NextTask)
		assert.Equal(t, "", status.NextTaskFeedback)
	})
}

// mockTaskStore is a test double for taskstore.Store.
type mockTaskStore struct {
	tasks   []*taskstore.Task
	listErr error
}

func (m *mockTaskStore) Get(id string) (*taskstore.Task, error) {
	for _, t := range m.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, &taskstore.NotFoundError{ID: id}
}

func (m *mockTaskStore) List() ([]*taskstore.Task, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tasks, nil
}

func (m *mockTaskStore) ListByParent(parentID string) ([]*taskstore.Task, error) {
	var result []*taskstore.Task
	for _, t := range m.tasks {
		if t.ParentID != nil && *t.ParentID == parentID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskStore) Save(task *taskstore.Task) error {
	return nil
}

func (m *mockTaskStore) UpdateStatus(id string, status taskstore.TaskStatus) error {
	return nil
}

func (m *mockTaskStore) Delete(id string) error {
	return nil
}
