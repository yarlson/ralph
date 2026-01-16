package selector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

// makeTaskWithLabels creates a task with optional labels.
func makeTaskWithLabels(id string, status taskstore.TaskStatus, parentID *string, dependsOn []string, labels map[string]string) *taskstore.Task {
	now := time.Now()
	return &taskstore.Task{
		ID:        id,
		Title:     "Task " + id,
		Status:    status,
		ParentID:  parentID,
		DependsOn: dependsOn,
		Labels:    labels,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// makeTaskWithTime creates a task with specific createdAt time.
func makeTaskWithTime(id string, status taskstore.TaskStatus, parentID *string, dependsOn []string, createdAt time.Time) *taskstore.Task {
	return &taskstore.Task{
		ID:        id,
		Title:     "Task " + id,
		Status:    status,
		ParentID:  parentID,
		DependsOn: dependsOn,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func TestSelectNext_NoReadyLeaves(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusOpen, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// dep-1 is the only ready leaf, but we're looking for descendants of "root" which doesn't exist
	selected := SelectNext(tasks, graph, "nonexistent-parent", nil)
	assert.Nil(t, selected, "should return nil when parent has no descendants")
}

func TestSelectNext_SingleReadyLeaf(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("child", taskstore.StatusOpen, strPtr("root"), nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "child", selected.ID)
}

func TestSelectNext_MultipleReadyLeaves_DeterministicOrder(t *testing.T) {
	// Create tasks with specific creation times to test deterministic ordering
	baseTime := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)

	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTaskWithTime("child-b", taskstore.StatusOpen, strPtr("root"), nil, baseTime.Add(2*time.Hour)),
		makeTaskWithTime("child-a", taskstore.StatusOpen, strPtr("root"), nil, baseTime.Add(1*time.Hour)),
		makeTaskWithTime("child-c", taskstore.StatusOpen, strPtr("root"), nil, baseTime.Add(3*time.Hour)),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// Without area preference, should select by createdAt, then ID
	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "child-a", selected.ID, "should select task with earliest createdAt")
}

func TestSelectNext_MultipleReadyLeaves_SameCreatedAt_OrderByID(t *testing.T) {
	// Create tasks with same creation time to test ID ordering fallback
	sameTime := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)

	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTaskWithTime("child-z", taskstore.StatusOpen, strPtr("root"), nil, sameTime),
		makeTaskWithTime("child-a", taskstore.StatusOpen, strPtr("root"), nil, sameTime),
		makeTaskWithTime("child-m", taskstore.StatusOpen, strPtr("root"), nil, sameTime),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "child-a", selected.ID, "should select task with alphabetically first ID when createdAt is equal")
}

func TestSelectNext_AreaPreference_SameAreaAsLastCompleted(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTaskWithLabels("task-core", taskstore.StatusOpen, strPtr("root"), nil, map[string]string{"area": "core"}),
		makeTaskWithLabels("task-cli", taskstore.StatusOpen, strPtr("root"), nil, map[string]string{"area": "cli"}),
		makeTaskWithLabels("task-core-2", taskstore.StatusOpen, strPtr("root"), nil, map[string]string{"area": "core"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// Last completed was in "core" area
	lastCompleted := makeTaskWithLabels("prev-task", taskstore.StatusCompleted, strPtr("root"), nil, map[string]string{"area": "core"})

	selected := SelectNext(tasks, graph, "root", lastCompleted)
	require.NotNil(t, selected)

	// Should prefer tasks with area=core
	assert.Equal(t, "core", selected.Labels["area"], "should select task with same area as last completed")
}

func TestSelectNext_AreaPreference_FallbackWhenNoMatch(t *testing.T) {
	baseTime := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)

	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		{
			ID:        "task-cli-1",
			Title:     "Task task-cli-1",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("root"),
			Labels:    map[string]string{"area": "cli"},
			CreatedAt: baseTime.Add(2 * time.Hour),
			UpdatedAt: baseTime.Add(2 * time.Hour),
		},
		{
			ID:        "task-cli-2",
			Title:     "Task task-cli-2",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("root"),
			Labels:    map[string]string{"area": "cli"},
			CreatedAt: baseTime.Add(1 * time.Hour),
			UpdatedAt: baseTime.Add(1 * time.Hour),
		},
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// Last completed was in "core" area, but no core tasks available
	lastCompleted := makeTaskWithLabels("prev-task", taskstore.StatusCompleted, strPtr("root"), nil, map[string]string{"area": "core"})

	selected := SelectNext(tasks, graph, "root", lastCompleted)
	require.NotNil(t, selected)

	// Should fall back to deterministic ordering (createdAt, then ID)
	assert.Equal(t, "task-cli-2", selected.ID, "should fall back to deterministic ordering when no area match")
}

func TestSelectNext_DescendantsOnly(t *testing.T) {
	// Tasks outside the parent hierarchy should not be selected
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("child", taskstore.StatusOpen, strPtr("root"), nil),
		makeTask("other-root", taskstore.StatusOpen, nil, nil),
		makeTask("other-child", taskstore.StatusOpen, strPtr("other-root"), nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "child", selected.ID, "should only select from descendants of parent")
}

func TestSelectNext_DeepHierarchy(t *testing.T) {
	// root -> parent -> grandchild (only grandchild is leaf)
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("parent", taskstore.StatusOpen, strPtr("root"), nil),
		makeTask("grandchild", taskstore.StatusOpen, strPtr("parent"), nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "grandchild", selected.ID, "should find leaf in deep hierarchy")
}

func TestSelectNext_WithDependencies(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("dep", taskstore.StatusCompleted, strPtr("root"), nil),
		makeTask("task-a", taskstore.StatusOpen, strPtr("root"), []string{"dep"}),
		makeTask("task-b", taskstore.StatusOpen, strPtr("root"), []string{"task-a"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "task-a", selected.ID, "should select task with completed dependencies")
}

func TestSelectNext_AllTasksCompleted(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("child-1", taskstore.StatusCompleted, strPtr("root"), nil),
		makeTask("child-2", taskstore.StatusCompleted, strPtr("root"), nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	selected := SelectNext(tasks, graph, "root", nil)
	assert.Nil(t, selected, "should return nil when all tasks are completed")
}

func TestSelectNext_NilLastCompleted(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("child", taskstore.StatusOpen, strPtr("root"), nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// Should work fine with nil lastCompleted
	selected := SelectNext(tasks, graph, "root", nil)
	require.NotNil(t, selected)
	assert.Equal(t, "child", selected.ID)
}

func TestSelectNext_LastCompletedNoLabels(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTaskWithLabels("task-1", taskstore.StatusOpen, strPtr("root"), nil, map[string]string{"area": "core"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// Last completed has no labels (no area)
	lastCompleted := makeTask("prev", taskstore.StatusCompleted, strPtr("root"), nil)

	selected := SelectNext(tasks, graph, "root", lastCompleted)
	require.NotNil(t, selected)
	assert.Equal(t, "task-1", selected.ID, "should use deterministic ordering when lastCompleted has no area")
}

func TestSelectNext_EmptyParentID(t *testing.T) {
	// When parentID is empty, should look for root-level tasks (no parent)
	tasks := []*taskstore.Task{
		makeTask("root-task-1", taskstore.StatusOpen, nil, nil),
		makeTask("root-task-2", taskstore.StatusOpen, nil, nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	// Empty parent should not match anything that has a parent
	selected := SelectNext(tasks, graph, "", nil)
	assert.Nil(t, selected, "should return nil for empty parentID (use case is to always specify a parent)")
}

func TestSelectNext_AreaPreferenceTieBreaker(t *testing.T) {
	// When multiple tasks have the same area, use deterministic ordering within that area
	baseTime := time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC)

	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		{
			ID:        "core-z",
			Title:     "Task core-z",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("root"),
			Labels:    map[string]string{"area": "core"},
			CreatedAt: baseTime.Add(2 * time.Hour),
			UpdatedAt: baseTime.Add(2 * time.Hour),
		},
		{
			ID:        "core-a",
			Title:     "Task core-a",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("root"),
			Labels:    map[string]string{"area": "core"},
			CreatedAt: baseTime.Add(1 * time.Hour),
			UpdatedAt: baseTime.Add(1 * time.Hour),
		},
		{
			ID:        "cli-early",
			Title:     "Task cli-early",
			Status:    taskstore.StatusOpen,
			ParentID:  strPtr("root"),
			Labels:    map[string]string{"area": "cli"},
			CreatedAt: baseTime,
			UpdatedAt: baseTime,
		},
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	lastCompleted := makeTaskWithLabels("prev", taskstore.StatusCompleted, strPtr("root"), nil, map[string]string{"area": "core"})

	selected := SelectNext(tasks, graph, "root", lastCompleted)
	require.NotNil(t, selected)
	// Should select core-a because:
	// 1. It matches the preferred area (core)
	// 2. Among core tasks, it has the earliest createdAt
	assert.Equal(t, "core-a", selected.ID, "should use deterministic ordering within preferred area")
}
