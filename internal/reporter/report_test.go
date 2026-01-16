package reporter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/go-ralph/internal/loop"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func TestReportDefaults(t *testing.T) {
	report := &Report{}

	assert.Empty(t, report.ParentTaskID)
	assert.Empty(t, report.FeatureName)
	assert.Nil(t, report.Commits)
	assert.Nil(t, report.CompletedTasks)
	assert.Nil(t, report.BlockedTasks)
	assert.Nil(t, report.FailedTasks)
	assert.Zero(t, report.TotalIterations)
	assert.Zero(t, report.TotalCostUSD)
	assert.Zero(t, report.TotalDuration)
	assert.True(t, report.StartTime.IsZero())
	assert.True(t, report.EndTime.IsZero())
}

func TestReportAllFields(t *testing.T) {
	now := time.Now()
	report := &Report{
		ParentTaskID: "parent-1",
		FeatureName:  "Feature X",
		Commits: []CommitInfo{
			{Hash: "abc123", Message: "feat: Add feature", TaskID: "task-1", Timestamp: now},
		},
		CompletedTasks: []TaskSummary{
			{ID: "task-1", Title: "Task 1", Outcome: "success"},
		},
		BlockedTasks: []BlockedTaskSummary{
			{ID: "task-2", Title: "Task 2", Reason: "dependency not met"},
		},
		FailedTasks: []TaskSummary{
			{ID: "task-3", Title: "Task 3", Outcome: "failed"},
		},
		TotalIterations: 5,
		TotalCostUSD:    1.23,
		TotalDuration:   10 * time.Minute,
		StartTime:       now.Add(-10 * time.Minute),
		EndTime:         now,
	}

	assert.Equal(t, "parent-1", report.ParentTaskID)
	assert.Equal(t, "Feature X", report.FeatureName)
	assert.Len(t, report.Commits, 1)
	assert.Len(t, report.CompletedTasks, 1)
	assert.Len(t, report.BlockedTasks, 1)
	assert.Len(t, report.FailedTasks, 1)
	assert.Equal(t, 5, report.TotalIterations)
	assert.Equal(t, 1.23, report.TotalCostUSD)
	assert.Equal(t, 10*time.Minute, report.TotalDuration)
}

func TestCommitInfoDefaults(t *testing.T) {
	ci := CommitInfo{}

	assert.Empty(t, ci.Hash)
	assert.Empty(t, ci.Message)
	assert.Empty(t, ci.TaskID)
	assert.True(t, ci.Timestamp.IsZero())
}

func TestTaskSummaryDefaults(t *testing.T) {
	ts := TaskSummary{}

	assert.Empty(t, ts.ID)
	assert.Empty(t, ts.Title)
	assert.Empty(t, ts.Outcome)
}

func TestBlockedTaskSummaryDefaults(t *testing.T) {
	bts := BlockedTaskSummary{}

	assert.Empty(t, bts.ID)
	assert.Empty(t, bts.Title)
	assert.Empty(t, bts.Reason)
}

func TestNewReportGenerator(t *testing.T) {
	store := &mockStore{tasks: []*taskstore.Task{}}
	logsDir := "/tmp/logs"

	gen := NewReportGenerator(store, logsDir)

	assert.NotNil(t, gen)
}

func TestGenerateReportNoTasks(t *testing.T) {
	store := &mockStore{tasks: []*taskstore.Task{}}

	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	assert.Equal(t, "parent-1", report.ParentTaskID)
	assert.Empty(t, report.Commits)
	assert.Empty(t, report.CompletedTasks)
	assert.Empty(t, report.BlockedTasks)
	assert.Empty(t, report.FailedTasks)
	assert.Zero(t, report.TotalIterations)
	assert.Zero(t, report.TotalCostUSD)
}

func TestGenerateReportWithCompletedTasks(t *testing.T) {
	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-2",
			Title:     "Task 2",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	assert.Len(t, report.CompletedTasks, 2)
	assert.Equal(t, "task-1", report.CompletedTasks[0].ID)
	assert.Equal(t, "Task 1", report.CompletedTasks[0].Title)
}

func TestGenerateReportWithBlockedTasks(t *testing.T) {
	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusBlocked,
			DependsOn: []string{"task-0"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	assert.Len(t, report.BlockedTasks, 1)
	assert.Equal(t, "task-1", report.BlockedTasks[0].ID)
	assert.Equal(t, "Task 1", report.BlockedTasks[0].Title)
	assert.Contains(t, report.BlockedTasks[0].Reason, "blocked")
}

func TestGenerateReportWithFailedTasks(t *testing.T) {
	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusFailed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	assert.Len(t, report.FailedTasks, 1)
	assert.Equal(t, "task-1", report.FailedTasks[0].ID)
	assert.Equal(t, "Task 1", report.FailedTasks[0].Title)
}

func TestGenerateReportWithSkippedTasks(t *testing.T) {
	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusSkipped,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	// Skipped tasks should be tracked separately
	assert.Len(t, report.SkippedTasks, 1)
	assert.Equal(t, "task-1", report.SkippedTasks[0].ID)
}

func TestGenerateReportWithIterationRecords(t *testing.T) {
	logsDir := t.TempDir()

	// Create iteration records
	record1 := &loop.IterationRecord{
		IterationID:  "iter-1",
		TaskID:       "task-1",
		StartTime:    time.Now().Add(-30 * time.Minute),
		EndTime:      time.Now().Add(-20 * time.Minute),
		Outcome:      loop.OutcomeSuccess,
		ResultCommit: "abc123",
		ClaudeInvocation: loop.ClaudeInvocationMeta{
			TotalCostUSD: 0.50,
		},
	}
	record2 := &loop.IterationRecord{
		IterationID:  "iter-2",
		TaskID:       "task-2",
		StartTime:    time.Now().Add(-20 * time.Minute),
		EndTime:      time.Now().Add(-10 * time.Minute),
		Outcome:      loop.OutcomeSuccess,
		ResultCommit: "def456",
		ClaudeInvocation: loop.ClaudeInvocationMeta{
			TotalCostUSD: 0.75,
		},
	}

	_, err := loop.SaveRecord(logsDir, record1)
	require.NoError(t, err)
	_, err = loop.SaveRecord(logsDir, record2)
	require.NoError(t, err)

	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-2",
			Title:     "Task 2",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, logsDir)

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	assert.Equal(t, 2, report.TotalIterations)
	assert.Equal(t, 1.25, report.TotalCostUSD)
	assert.Len(t, report.Commits, 2)
}

func TestGenerateReportCommitsFromRecords(t *testing.T) {
	logsDir := t.TempDir()

	timestamp := time.Now().Add(-10 * time.Minute)
	record := &loop.IterationRecord{
		IterationID:  "iter-1",
		TaskID:       "task-1",
		StartTime:    timestamp,
		EndTime:      timestamp.Add(5 * time.Minute),
		Outcome:      loop.OutcomeSuccess,
		ResultCommit: "abc123def456",
	}

	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, logsDir)

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	require.Len(t, report.Commits, 1)
	assert.Equal(t, "abc123def456", report.Commits[0].Hash)
	assert.Equal(t, "task-1", report.Commits[0].TaskID)
}

func TestGenerateReportMixedStatuses(t *testing.T) {
	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Completed Task",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-2",
			Title:     "Blocked Task",
			ParentID:  &parentID,
			Status:    taskstore.StatusBlocked,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-3",
			Title:     "Failed Task",
			ParentID:  &parentID,
			Status:    taskstore.StatusFailed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-4",
			Title:     "Skipped Task",
			ParentID:  &parentID,
			Status:    taskstore.StatusSkipped,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-5",
			Title:     "Open Task",
			ParentID:  &parentID,
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	assert.Len(t, report.CompletedTasks, 1)
	assert.Len(t, report.BlockedTasks, 1)
	assert.Len(t, report.FailedTasks, 1)
	assert.Len(t, report.SkippedTasks, 1)
}

func TestGenerateReportDeepHierarchy(t *testing.T) {
	parentID := "parent-1"
	childID := "child-1"
	tasks := []*taskstore.Task{
		{
			ID:        "child-1",
			Title:     "Child Container",
			ParentID:  &parentID,
			Status:    taskstore.StatusOpen,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "leaf-1",
			Title:     "Leaf Task 1",
			ParentID:  &childID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "leaf-2",
			Title:     "Leaf Task 2",
			ParentID:  &childID,
			Status:    taskstore.StatusFailed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, "")

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	// Should include all descendants
	assert.Len(t, report.CompletedTasks, 1)
	assert.Len(t, report.FailedTasks, 1)
}

func TestGenerateReportTimeRange(t *testing.T) {
	logsDir := t.TempDir()

	startTime := time.Now().Add(-60 * time.Minute)
	endTime := time.Now().Add(-10 * time.Minute)

	record1 := &loop.IterationRecord{
		IterationID: "iter-1",
		TaskID:      "task-1",
		StartTime:   startTime,
		EndTime:     startTime.Add(10 * time.Minute),
		Outcome:     loop.OutcomeSuccess,
	}
	record2 := &loop.IterationRecord{
		IterationID: "iter-2",
		TaskID:      "task-2",
		StartTime:   endTime.Add(-10 * time.Minute),
		EndTime:     endTime,
		Outcome:     loop.OutcomeSuccess,
	}

	_, err := loop.SaveRecord(logsDir, record1)
	require.NoError(t, err)
	_, err = loop.SaveRecord(logsDir, record2)
	require.NoError(t, err)

	parentID := "parent-1"
	tasks := []*taskstore.Task{
		{
			ID:        "task-1",
			Title:     "Task 1",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "task-2",
			Title:     "Task 2",
			ParentID:  &parentID,
			Status:    taskstore.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	store := &mockStore{tasks: tasks}
	gen := NewReportGenerator(store, logsDir)

	report, err := gen.GenerateReport("parent-1")
	require.NoError(t, err)

	// Use WithinDuration for time comparison to handle monotonic clock differences
	assert.WithinDuration(t, startTime, report.StartTime, time.Second)
	assert.WithinDuration(t, endTime, report.EndTime, time.Second)
	assert.InDelta(t, endTime.Sub(startTime), report.TotalDuration, float64(time.Second))
}

func TestLoadAllIterationRecords(t *testing.T) {
	logsDir := t.TempDir()

	// Create records
	record1 := &loop.IterationRecord{
		IterationID: "iter-1",
		TaskID:      "task-1",
		Outcome:     loop.OutcomeSuccess,
	}
	record2 := &loop.IterationRecord{
		IterationID: "iter-2",
		TaskID:      "task-2",
		Outcome:     loop.OutcomeFailed,
	}

	_, err := loop.SaveRecord(logsDir, record1)
	require.NoError(t, err)
	_, err = loop.SaveRecord(logsDir, record2)
	require.NoError(t, err)

	records, err := LoadAllIterationRecords(logsDir)
	require.NoError(t, err)

	assert.Len(t, records, 2)
}

func TestLoadAllIterationRecordsEmptyDir(t *testing.T) {
	logsDir := t.TempDir()

	records, err := LoadAllIterationRecords(logsDir)
	require.NoError(t, err)

	assert.Len(t, records, 0)
}

func TestLoadAllIterationRecordsNonExistentDir(t *testing.T) {
	records, err := LoadAllIterationRecords("/nonexistent/dir")
	require.NoError(t, err)

	assert.Len(t, records, 0)
}

func TestLoadAllIterationRecordsSkipsInvalidFiles(t *testing.T) {
	logsDir := t.TempDir()

	// Create valid record
	record := &loop.IterationRecord{
		IterationID: "iter-1",
		TaskID:      "task-1",
		Outcome:     loop.OutcomeSuccess,
	}
	_, err := loop.SaveRecord(logsDir, record)
	require.NoError(t, err)

	// Create invalid file
	invalidPath := filepath.Join(logsDir, "iteration-invalid.json")
	err = os.WriteFile(invalidPath, []byte("not json"), 0644)
	require.NoError(t, err)

	// Create non-iteration file
	otherPath := filepath.Join(logsDir, "other-file.json")
	err = os.WriteFile(otherPath, []byte("{}"), 0644)
	require.NoError(t, err)

	records, err := LoadAllIterationRecords(logsDir)
	require.NoError(t, err)

	// Should only include the valid iteration record
	assert.Len(t, records, 1)
}

func TestFormatReport(t *testing.T) {
	now := time.Now()
	report := &Report{
		ParentTaskID:    "parent-1",
		FeatureName:     "Feature X",
		TotalIterations: 3,
		TotalCostUSD:    1.50,
		TotalDuration:   15 * time.Minute,
		StartTime:       now.Add(-15 * time.Minute),
		EndTime:         now,
		Commits: []CommitInfo{
			{Hash: "abc123", Message: "feat: Task 1", TaskID: "task-1", Timestamp: now},
		},
		CompletedTasks: []TaskSummary{
			{ID: "task-1", Title: "Task 1", Outcome: "success"},
		},
		BlockedTasks: []BlockedTaskSummary{
			{ID: "task-2", Title: "Task 2", Reason: "dependency not met"},
		},
		FailedTasks: []TaskSummary{
			{ID: "task-3", Title: "Task 3", Outcome: "failed"},
		},
	}

	formatted := FormatReport(report)

	assert.Contains(t, formatted, "Feature Report")
	assert.Contains(t, formatted, "parent-1")
	assert.Contains(t, formatted, "Feature X")
	assert.Contains(t, formatted, "3 iterations")
	assert.Contains(t, formatted, "$1.50")
	assert.Contains(t, formatted, "Commits")
	assert.Contains(t, formatted, "abc123")
	assert.Contains(t, formatted, "Completed Tasks")
	assert.Contains(t, formatted, "task-1")
	assert.Contains(t, formatted, "Blocked Tasks")
	assert.Contains(t, formatted, "task-2")
	assert.Contains(t, formatted, "dependency not met")
	assert.Contains(t, formatted, "Failed Tasks")
	assert.Contains(t, formatted, "task-3")
}

func TestFormatReportMinimal(t *testing.T) {
	report := &Report{
		ParentTaskID: "parent-1",
	}

	formatted := FormatReport(report)

	assert.Contains(t, formatted, "Feature Report")
	assert.Contains(t, formatted, "parent-1")
	assert.Contains(t, formatted, "No commits")
	assert.Contains(t, formatted, "No completed tasks")
}

func TestFormatReportWithSkippedTasks(t *testing.T) {
	report := &Report{
		ParentTaskID: "parent-1",
		SkippedTasks: []TaskSummary{
			{ID: "task-1", Title: "Skipped Task"},
		},
	}

	formatted := FormatReport(report)

	assert.Contains(t, formatted, "Skipped Tasks")
	assert.Contains(t, formatted, "task-1")
}

// mockStore implements taskstore.Store for testing
type mockStore struct {
	tasks []*taskstore.Task
}

func (m *mockStore) Get(id string) (*taskstore.Task, error) {
	for _, t := range m.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, taskstore.ErrNotFound
}

func (m *mockStore) List() ([]*taskstore.Task, error) {
	return m.tasks, nil
}

func (m *mockStore) ListByParent(parentID string) ([]*taskstore.Task, error) {
	var result []*taskstore.Task
	for _, t := range m.tasks {
		if t.ParentID != nil && *t.ParentID == parentID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockStore) Save(task *taskstore.Task) error {
	for i, t := range m.tasks {
		if t.ID == task.ID {
			m.tasks[i] = task
			return nil
		}
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *mockStore) UpdateStatus(id string, status taskstore.TaskStatus) error {
	for _, t := range m.tasks {
		if t.ID == id {
			t.Status = status
			return nil
		}
	}
	return taskstore.ErrNotFound
}

func (m *mockStore) Delete(id string) error {
	for i, t := range m.tasks {
		if t.ID == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			return nil
		}
	}
	return taskstore.ErrNotFound
}
