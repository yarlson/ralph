package selector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/go-ralph/internal/taskstore"
)

func makeTask(id string, status taskstore.TaskStatus, parentID *string, dependsOn []string) *taskstore.Task {
	now := time.Now()
	return &taskstore.Task{
		ID:        id,
		Title:     "Task " + id,
		Status:    status,
		ParentID:  parentID,
		DependsOn: dependsOn,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func strPtr(s string) *string {
	return &s
}

func TestComputeReady_NoDependencies(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("task-1", taskstore.StatusOpen, nil, nil),
		makeTask("task-2", taskstore.StatusOpen, nil, nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	ready := ComputeReady(tasks, graph)

	assert.True(t, ready["task-1"], "task-1 should be ready (no dependencies)")
	assert.True(t, ready["task-2"], "task-2 should be ready (no dependencies)")
}

func TestComputeReady_WithCompletedDependencies(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusCompleted, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	ready := ComputeReady(tasks, graph)

	assert.True(t, ready["dep-1"], "dep-1 should be ready (no dependencies)")
	assert.True(t, ready["task-1"], "task-1 should be ready (dependency completed)")
}

func TestComputeReady_WithIncompleteDependencies(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusOpen, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	ready := ComputeReady(tasks, graph)

	assert.True(t, ready["dep-1"], "dep-1 should be ready (no dependencies)")
	assert.False(t, ready["task-1"], "task-1 should NOT be ready (dependency not completed)")
}

func TestComputeReady_MultipleDependencies(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusCompleted, nil, nil),
		makeTask("dep-2", taskstore.StatusOpen, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1", "dep-2"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	ready := ComputeReady(tasks, graph)

	assert.True(t, ready["dep-1"], "dep-1 should be ready")
	assert.True(t, ready["dep-2"], "dep-2 should be ready")
	assert.False(t, ready["task-1"], "task-1 should NOT be ready (dep-2 not completed)")
}

func TestComputeReady_AllDependenciesCompleted(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusCompleted, nil, nil),
		makeTask("dep-2", taskstore.StatusCompleted, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1", "dep-2"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	ready := ComputeReady(tasks, graph)

	assert.True(t, ready["task-1"], "task-1 should be ready (all dependencies completed)")
}

func TestComputeReady_TransitiveDependencies(t *testing.T) {
	// dep-1 -> dep-2 -> task-1
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusOpen, nil, nil),
		makeTask("dep-2", taskstore.StatusOpen, nil, []string{"dep-1"}),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-2"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	ready := ComputeReady(tasks, graph)

	assert.True(t, ready["dep-1"], "dep-1 should be ready (no dependencies)")
	assert.False(t, ready["dep-2"], "dep-2 should NOT be ready (dep-1 not completed)")
	assert.False(t, ready["task-1"], "task-1 should NOT be ready (dep-2 not completed)")
}

func TestIsLeaf_NoChildren(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("task-1", taskstore.StatusOpen, nil, nil),
	}

	isLeaf := IsLeaf(tasks, "task-1")
	assert.True(t, isLeaf, "task-1 should be a leaf (no children)")
}

func TestIsLeaf_WithChildren(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("parent", taskstore.StatusOpen, nil, nil),
		makeTask("child-1", taskstore.StatusOpen, strPtr("parent"), nil),
		makeTask("child-2", taskstore.StatusOpen, strPtr("parent"), nil),
	}

	assert.False(t, IsLeaf(tasks, "parent"), "parent should NOT be a leaf (has children)")
	assert.True(t, IsLeaf(tasks, "child-1"), "child-1 should be a leaf")
	assert.True(t, IsLeaf(tasks, "child-2"), "child-2 should be a leaf")
}

func TestIsLeaf_NestedHierarchy(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("parent", taskstore.StatusOpen, strPtr("root"), nil),
		makeTask("leaf", taskstore.StatusOpen, strPtr("parent"), nil),
	}

	assert.False(t, IsLeaf(tasks, "root"), "root should NOT be a leaf")
	assert.False(t, IsLeaf(tasks, "parent"), "parent should NOT be a leaf")
	assert.True(t, IsLeaf(tasks, "leaf"), "leaf should be a leaf")
}

func TestIsLeaf_NonexistentTask(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("task-1", taskstore.StatusOpen, nil, nil),
	}

	// Non-existent tasks are considered leaves (they have no children)
	assert.True(t, IsLeaf(tasks, "nonexistent"), "nonexistent task should be considered a leaf")
}

func TestGetReadyLeaves_SimpleCase(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("task-1", taskstore.StatusOpen, nil, nil),
		makeTask("task-2", taskstore.StatusOpen, nil, nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	assert.Len(t, leaves, 2)
	assert.Contains(t, leaves, tasks[0])
	assert.Contains(t, leaves, tasks[1])
}

func TestGetReadyLeaves_FiltersByStatus(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("completed", taskstore.StatusCompleted, nil, nil),
		makeTask("in-progress", taskstore.StatusInProgress, nil, nil),
		makeTask("blocked", taskstore.StatusBlocked, nil, nil),
		makeTask("open", taskstore.StatusOpen, nil, nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	assert.Len(t, leaves, 1)
	assert.Equal(t, "open", leaves[0].ID)
}

func TestGetReadyLeaves_FiltersByReady(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusOpen, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	assert.Len(t, leaves, 1)
	assert.Equal(t, "dep-1", leaves[0].ID)
}

func TestGetReadyLeaves_FiltersByLeaf(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("parent", taskstore.StatusOpen, nil, nil),
		makeTask("child", taskstore.StatusOpen, strPtr("parent"), nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	assert.Len(t, leaves, 1)
	assert.Equal(t, "child", leaves[0].ID)
}

func TestGetReadyLeaves_ComplexScenario(t *testing.T) {
	// Hierarchy:
	// root (parent)
	//   ├── setup (completed leaf) -> not in result (completed)
	//   ├── feature (parent)
	//   │   ├── feat-1 (depends on setup, open) -> in result
	//   │   └── feat-2 (depends on feat-1, open) -> not ready
	//   └── cleanup (depends on feature, open) -> not ready (dependency on parent)

	tasks := []*taskstore.Task{
		makeTask("root", taskstore.StatusOpen, nil, nil),
		makeTask("setup", taskstore.StatusCompleted, strPtr("root"), nil),
		makeTask("feature", taskstore.StatusOpen, strPtr("root"), nil),
		makeTask("feat-1", taskstore.StatusOpen, strPtr("feature"), []string{"setup"}),
		makeTask("feat-2", taskstore.StatusOpen, strPtr("feature"), []string{"feat-1"}),
		makeTask("cleanup", taskstore.StatusOpen, strPtr("root"), []string{"feature"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	// Only feat-1 should be returned:
	// - root: not a leaf (has children)
	// - setup: completed
	// - feature: not a leaf (has children)
	// - feat-1: open, ready (setup completed), leaf
	// - feat-2: not ready (feat-1 not completed)
	// - cleanup: not ready (feature not completed)
	assert.Len(t, leaves, 1)
	assert.Equal(t, "feat-1", leaves[0].ID)
}

func TestGetReadyLeaves_EmptyList(t *testing.T) {
	tasks := []*taskstore.Task{}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	assert.Empty(t, leaves)
}

func TestGetReadyLeaves_AllCompleted(t *testing.T) {
	tasks := []*taskstore.Task{
		makeTask("task-1", taskstore.StatusCompleted, nil, nil),
		makeTask("task-2", taskstore.StatusCompleted, nil, nil),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	assert.Empty(t, leaves)
}

func TestGetReadyLeaves_AllBlocked(t *testing.T) {
	// All tasks depend on incomplete dependencies
	tasks := []*taskstore.Task{
		makeTask("dep-1", taskstore.StatusFailed, nil, nil),
		makeTask("task-1", taskstore.StatusOpen, nil, []string{"dep-1"}),
		makeTask("task-2", taskstore.StatusOpen, nil, []string{"dep-1"}),
	}

	graph, err := BuildGraph(tasks)
	require.NoError(t, err)

	leaves := GetReadyLeaves(tasks, graph)

	// Only dep-1 could be selected but it's failed status, which is NOT open
	// So no tasks are ready
	assert.Empty(t, leaves)
}
