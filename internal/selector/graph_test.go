package selector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/ralph/internal/taskstore"
)

func TestBuildGraph_EmptyTaskList(t *testing.T) {
	g, err := BuildGraph([]*taskstore.Task{})
	require.NoError(t, err)
	assert.NotNil(t, g)
	assert.Empty(t, g.Nodes())
}

func TestBuildGraph_SingleTask(t *testing.T) {
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)
	assert.Equal(t, []string{"task-1"}, g.Nodes())
	assert.Empty(t, g.Dependencies("task-1"))
}

func TestBuildGraph_LinearDependencies(t *testing.T) {
	// task-3 depends on task-2, task-2 depends on task-1
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-2"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)
	assert.Len(t, g.Nodes(), 3)
	assert.Empty(t, g.Dependencies("task-1"))
	assert.Equal(t, []string{"task-1"}, g.Dependencies("task-2"))
	assert.Equal(t, []string{"task-2"}, g.Dependencies("task-3"))
}

func TestBuildGraph_MultipleDependencies(t *testing.T) {
	// task-3 depends on both task-1 and task-2
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2"},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1", "task-2"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)
	deps := g.Dependencies("task-3")
	assert.Len(t, deps, 2)
	assert.Contains(t, deps, "task-1")
	assert.Contains(t, deps, "task-2")
}

func TestBuildGraph_DiamondDependencies(t *testing.T) {
	// Diamond pattern: task-4 depends on task-2 and task-3, both depend on task-1
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1"}},
		{ID: "task-4", Title: "Task 4", DependsOn: []string{"task-2", "task-3"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)
	assert.Len(t, g.Nodes(), 4)
}

func TestDetectCycles_NoCycle(t *testing.T) {
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-2"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	cycle := g.DetectCycle()
	assert.Nil(t, cycle)
}

func TestDetectCycles_SelfCycle(t *testing.T) {
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1", DependsOn: []string{"task-1"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	cycle := g.DetectCycle()
	require.NotNil(t, cycle)
	assert.Contains(t, cycle, "task-1")
}

func TestDetectCycles_TwoNodeCycle(t *testing.T) {
	// task-1 depends on task-2, task-2 depends on task-1
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1", DependsOn: []string{"task-2"}},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	cycle := g.DetectCycle()
	require.NotNil(t, cycle)
	assert.GreaterOrEqual(t, len(cycle), 2)
}

func TestDetectCycles_ThreeNodeCycle(t *testing.T) {
	// task-1 -> task-2 -> task-3 -> task-1
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1", DependsOn: []string{"task-3"}},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-2"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	cycle := g.DetectCycle()
	require.NotNil(t, cycle)
	assert.GreaterOrEqual(t, len(cycle), 3)
}

func TestDetectCycles_CycleInSubgraph(t *testing.T) {
	// task-1 is fine, but task-2 and task-3 form a cycle
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1", "task-3"}},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-2"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	cycle := g.DetectCycle()
	require.NotNil(t, cycle)
	// Cycle should include task-2 and task-3, not task-1
	assert.Contains(t, cycle, "task-2")
	assert.Contains(t, cycle, "task-3")
}

func TestGraph_Dependents(t *testing.T) {
	// task-2 and task-3 depend on task-1
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	dependents := g.Dependents("task-1")
	assert.Len(t, dependents, 2)
	assert.Contains(t, dependents, "task-2")
	assert.Contains(t, dependents, "task-3")
}

func TestGraph_HasNode(t *testing.T) {
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	assert.True(t, g.HasNode("task-1"))
	assert.False(t, g.HasNode("task-2"))
}

func TestGraph_TopologicalSort_NoCycle(t *testing.T) {
	// task-3 depends on task-2, task-2 depends on task-1
	tasks := []*taskstore.Task{
		{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-2"}},
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	sorted, err := g.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	// task-1 must come before task-2, task-2 must come before task-3
	idx := make(map[string]int)
	for i, id := range sorted {
		idx[id] = i
	}
	assert.Less(t, idx["task-1"], idx["task-2"])
	assert.Less(t, idx["task-2"], idx["task-3"])
}

func TestGraph_TopologicalSort_WithCycle(t *testing.T) {
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1", DependsOn: []string{"task-2"}},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
	}

	g, err := BuildGraph(tasks)
	require.NoError(t, err)

	_, err = g.TopologicalSort()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestBuildGraph_MissingDependency(t *testing.T) {
	// task-2 depends on task-3, but task-3 doesn't exist
	tasks := []*taskstore.Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-3"}},
	}

	_, err := BuildGraph(tasks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task-3")
	assert.Contains(t, err.Error(), "task-2")
}
